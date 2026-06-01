package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/E-cercise/E-cercise/src/config"
	"github.com/E-cercise/E-cercise/src/logger"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"mime/multipart"
)

type CloudinaryService interface {
	UploadImage(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader, fileName string) (string, error)
	DeleteImage(ctx context.Context, publicID string) error
	MoveImage(ctx context.Context, publicID, newFolder string) error
}

type cloudinaryService struct {
	cloudinary *cloudinary.Cloudinary
}

func NewCloudinaryService() (CloudinaryService, error) {

	if config.CloudinaryCloudName == "" || config.CloudinaryApiKey == "" || config.CloudinaryApiSecret == "" {
		return nil, fmt.Errorf("missing Cloudinary environment variables")
	}

	// Initialize Cloudinary instance
	cld, err := cloudinary.NewFromParams(config.CloudinaryCloudName, config.CloudinaryApiKey, config.CloudinaryApiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cloudinary: %w", err)
	}

	return &cloudinaryService{cloudinary: cld}, nil
}

// UploadImage uploads an image file to Cloudinary and returns the URL
func (s *cloudinaryService) UploadImage(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader, fileName string) (string, error) {
	// Validate file type
	allowedTypes := []string{"image/jpeg", "image/png", "image/heic"}
	if err := validateFileType(fileHeader, allowedTypes); err != nil {
		return "", fmt.Errorf("file validation failed: %w", err)
	}

	// Validate file size
	const maxFileSize = 5 * 1024 * 1024 // 5 MB
	if err := validateFileSize(fileHeader, maxFileSize); err != nil {
		return "", fmt.Errorf("file validation failed: %w", err)
	}

	// Upload parameters
	uploadParams := uploader.UploadParams{
		PublicID: fileName,
	}

	// Log the file name
	logger.Log.Infof("Uploading file: %s", fileHeader.Filename)

	// Upload the file
	resp, err := s.cloudinary.Upload.Upload(ctx, file, uploadParams)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	if resp.Error.Message != "" {
		logger.Log.Errorf("Cloudinary API error: %s", resp.Error.Message)
		return "", fmt.Errorf("cloudinary API error: %s", resp.Error.Message)
	}

	if resp.SecureURL == "" {
		return "", errors.New("cloudinary response SecureURL is null")
	}

	return resp.SecureURL, nil
}

// DeleteImage deletes an image from Cloudinary by public ID
func (s *cloudinaryService) DeleteImage(ctx context.Context, publicID string) error {
	_, err := s.cloudinary.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: publicID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	return nil
}

// MoveImage moves an image from one folder to another on Cloudinary
func (s *cloudinaryService) MoveImage(ctx context.Context, fromPublicID, toPublicID string) error {

	resp, err := s.cloudinary.Upload.Rename(ctx, uploader.RenameParams{
		FromPublicID: fromPublicID,
		ToPublicID:   toPublicID,
	})
	if err != nil {
		return fmt.Errorf("failed to move image: %w", err)
	}

	if resp.Error != nil && resp.Error != "" {
		logger.Log.Errorf("Cloudinary API error: %v", resp.Error)
		return fmt.Errorf("cloudinary API error: %v", resp.Error)
	}

	return nil
}

func validateFileType(fileHeader *multipart.FileHeader, allowedTypes []string) error {
	f, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open file for type detection: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := io.ReadAtLeast(f, buf, 1)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("failed to read file for type detection: %w", err)
	}
	detectedType := http.DetectContentType(buf[:n])

	for _, allowedType := range allowedTypes {
		if detectedType == allowedType {
			return nil
		}
	}
	// http.DetectContentType has no HEIC signature (returns octet-stream). iOS
	// camera photos are HEIC, so sniff the ISO-BMFF 'ftyp' box directly so they
	// aren't rejected when image/heic is in the allow-list.
	if n >= 12 && string(buf[4:8]) == "ftyp" {
		for _, allowedType := range allowedTypes {
			if allowedType == "image/heic" {
				return nil
			}
		}
	}
	return fmt.Errorf("invalid file type: %s", detectedType)
}

func validateFileSize(fileHeader *multipart.FileHeader, maxSize int64) error {
	if fileHeader.Size > maxSize {
		return fmt.Errorf("file size exceeds limit: %d bytes", maxSize)
	}
	return nil
}
