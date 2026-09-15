package challenge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type capVerifier struct {
	endpoint string
	secret   string
	client   *http.Client
}

type capVerifyResponse struct {
	Success bool `json:"success"`
}

func (verifier *capVerifier) Verify(ctx context.Context, token, _ string) error {
	body, err := json.Marshal(map[string]string{
		"secret":   verifier.secret,
		"response": token,
	})
	if err != nil {
		return ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, verifier.endpoint+"/siteverify", bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := verifier.client.Do(request)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 64<<10)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, limited)
		return ErrUnavailable
	}
	var decoded capVerifyResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return ErrUnavailable
	}
	if !decoded.Success {
		return ErrInvalid
	}
	return nil
}
