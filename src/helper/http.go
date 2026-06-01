package helper

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// PostJSON sends a POST request with a JSON body and the given extra headers,
// returning the response body as []byte. Headers are passed PER CALL so a
// secret meant for one target isn't broadcast to every PostJSON destination.
func PostJSON(url string, payload interface{}, headers map[string]string) ([]byte, error) {
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
	for k, v := range headers {
		req.Header.Set(k, v)
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
