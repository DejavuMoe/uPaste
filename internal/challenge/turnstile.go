package challenge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const turnstileSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type turnstileVerifier struct {
	secret        string
	hostname      string
	client        *http.Client
	siteverifyURL string
}

type turnstileResponse struct {
	Success  bool     `json:"success"`
	Hostname string   `json:"hostname"`
	Action   string   `json:"action"`
	Errors   []string `json:"error-codes"`
}

func (verifier *turnstileVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	form := url.Values{}
	form.Set("secret", verifier.secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, verifier.siteverifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrUnavailable
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
	var decoded turnstileResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return ErrUnavailable
	}
	if !decoded.Success {
		return ErrInvalid
	}
	if !strings.EqualFold(decoded.Hostname, verifier.hostname) {
		return ErrInvalid
	}
	if decoded.Action != "create_share" {
		return ErrInvalid
	}
	return nil
}
