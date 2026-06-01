package helper

import (
	"github.com/E-cercise/E-cercise/src/model"
)

func FindPrimaryImageFromEquipment(equipment model.Equipment) *model.Image {
	if len(equipment.EquipmentOptions) == 0 {
		return nil
	}
	for _, image := range equipment.EquipmentOptions[0].Images {
		if image.IsPrimary {
			return &image
		}
	}
	return nil
}

func FindPrimaryImage(equipmentOption model.EquipmentOption) *model.Image {
	for _, image := range equipmentOption.Images {
		if image.IsPrimary {
			return &image
		}
	}
	return nil
}

// PrimaryImageURL returns the CloudinaryPath of the primary image in
// equipmentOption, or fallback if no primary image exists.
func PrimaryImageURL(opt model.EquipmentOption, fallback string) string {
	img := FindPrimaryImage(opt)
	if img == nil {
		return fallback
	}
	return img.CloudinaryPath
}

func GetMuscleGroupIDFromEquipment(equipment model.Equipment) []string {
	var muscleGroups []string
	for _, musGroup := range equipment.MuscleGroups {
		muscleGroups = append(muscleGroups, musGroup.ID)
	}
	return muscleGroups
}

func FindCommonAttributes(equipments []model.Equipment) []string {
	commonAttrs := make(map[string]string)

	// Initialize with first equipment's attributes
	for _, attr := range equipments[0].Attribute {
		commonAttrs[attr.Key] = attr.Value
	}

	// Compare with the other two
	for i := 1; i < len(equipments); i++ {
		eq := equipments[i]
		for key, _ := range commonAttrs {
			found := false
			for _, attr := range eq.Attribute {
				if attr.Key == key {
					found = true
					break
				}
			}
			if !found {
				delete(commonAttrs, key)
			}
		}
	}

	// Extract common keys
	commonKeys := []string{}
	for key := range commonAttrs {
		commonKeys = append(commonKeys, key)
	}
	return commonKeys
}
