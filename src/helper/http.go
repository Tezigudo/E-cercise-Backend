package helper

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/E-cercise/E-cercise/src/config"
)

// PostJSON sends a POST request with JSON body and returns the response body as []byte
func PostJSON(url string, payload interface{}) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Shared-secret auth for the internal recommender call (no-op if unset).
	if token := config.RecommenderInternalToken; token != "" {
		req.Header.Set("X-Internal-Token", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, &HttpError{StatusCode: resp.StatusCode, Message: string(b)}
	}

	return io.ReadAll(resp.Body)
}

// HttpError lets you capture HTTP-level errors (e.g. 500 or 400)
type HttpError struct {
	StatusCode int
	Message    string
}

func (e *HttpError) Error() string {
	return e.Message
}
