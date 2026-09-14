package fileapi

import (
	"net/http"
	"testing"

	"github.com/DejavuMoe/uPaste/internal/abuse"
)

func TestFileRateAndConcurrencyLimits(t *testing.T) {
	env := newTestEnv(t)
	value, _ := env.create(t, "x", "x", nil)
	control := abuse.New(abuse.Config{})
	env.handler = NewWithAbuse(env.service, env.log, control)
	for range 60 {
		if response := env.request(http.MethodGet, "/f/"+value.ID.String(), nil); response.Code != http.StatusOK {
			t.Fatal(response.Code)
		}
	}
	response := env.request(http.MethodHead, "/f/"+value.ID.String(), nil)
	if response.Code != http.StatusTooManyRequests || response.Body.Len() != 0 || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("rate status = %d", response.Code)
	}
	assertSecurity(t, response)
	control = abuse.New(abuse.Config{})
	for range abuse.DownloadLimit {
		if !control.TryDownload() {
			t.Fatal("slot")
		}
	}
	defer func() {
		for range abuse.DownloadLimit {
			control.ReleaseDownload()
		}
	}()
	env.handler = NewWithAbuse(env.service, env.log, control)
	response = env.request(http.MethodGet, "/f/"+value.ID.String(), nil)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("concurrency status = %d", response.Code)
	}
}
