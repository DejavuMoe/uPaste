package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DejavuMoe/uPaste/internal/webapp"
)

func TestHealthz(t *testing.T) {
	recorder := httptest.NewRecorder()
	newHandler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestApplicationHandlerDoesNotExposeFileRoutes(t *testing.T) {
	recorder := httptest.NewRecorder()
	newHandler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/f/AAAAAAAAAAAAAAAAAAAAAA", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestHealthzRejectsOtherMethods(t *testing.T) {
	recorder := httptest.NewRecorder()
	newHandler(nil, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func newTestFrontend(t *testing.T) *webapp.Handler {
	t.Helper()
	handler, err := webapp.New(fstest.MapFS{
		"index.html":              &fstest.MapFile{Data: []byte("<!doctype html><div id=\"root\"></div>")},
		"assets/index-test123.js": &fstest.MapFile{Data: []byte("console.log('ok')")},
	})
	if err != nil {
		t.Fatalf("webapp.New() error = %v", err)
	}
	return handler
}

func TestApplicationRoutingPrecedence(t *testing.T) {
	frontend := newTestFrontend(t)
	const apiMarker = "api-handler"
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, apiMarker)
	})
	handler := newHandler(api, frontend)

	tests := []struct {
		name        string
		method      string
		path        string
		status      int
		body        string
		contentType string
	}{
		{name: "healthz wins over frontend", method: http.MethodGet, path: "/healthz", status: http.StatusOK, body: "{\"status\":\"ok\"}\n"},
		{name: "healthz rejects post", method: http.MethodPost, path: "/healthz", status: http.StatusMethodNotAllowed},
		{name: "api reaches api handler", method: http.MethodGet, path: "/api/v1/shares/AAAAAAAAAAAAAAAAAAAAAA", status: http.StatusTeapot, body: apiMarker},
		{name: "api prefix does not reach frontend", method: http.MethodGet, path: "/api/not-frontend", status: http.StatusTeapot, body: apiMarker},
		{name: "raw reaches api handler", method: http.MethodGet, path: "/raw/AAAAAAAAAAAAAAAAAAAAAA", status: http.StatusTeapot, body: apiMarker},
		{name: "root serves frontend", method: http.MethodGet, path: "/", status: http.StatusOK, body: "root", contentType: "text/html; charset=utf-8"},
		{name: "viewer route serves frontend", method: http.MethodGet, path: "/s/AAAAAAAAAAAAAAAAAAAAAA", status: http.StatusOK, body: "root", contentType: "text/html; charset=utf-8"},
		{name: "management route serves frontend", method: http.MethodGet, path: "/manage/AAAAAAAAAAAAAAAAAAAAAA", status: http.StatusOK, body: "root", contentType: "text/html; charset=utf-8"},
		{name: "asset serves frontend", method: http.MethodGet, path: "/assets/index-test123.js", status: http.StatusOK, body: "console.log", contentType: "text/javascript; charset=utf-8"},
		{name: "unknown route is 404", method: http.MethodGet, path: "/does-not-exist", status: http.StatusNotFound, body: "404 page not found"},
		{name: "file route is 404 on app listener", method: http.MethodGet, path: "/f/AAAAAAAAAAAAAAAAAAAAAA", status: http.StatusNotFound, body: "404 page not found"},
		{name: "post to frontend route is 405", method: http.MethodPost, path: "/", status: http.StatusMethodNotAllowed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if test.body != "" && !strings.Contains(recorder.Body.String(), test.body) {
				t.Fatalf("body = %q, want substring %q", recorder.Body.String(), test.body)
			}
			if test.contentType != "" {
				if got := recorder.Header().Get("Content-Type"); got != test.contentType {
					t.Fatalf("Content-Type = %q, want %q", got, test.contentType)
				}
			}
		})
	}
}

func TestNilFrontendKeepsDevelopmentShape(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := newHandler(api, nil)
	for _, path := range []string{"/", "/s/AAAAAAAAAAAAAAAAAAAAAA", "/manage/AAAAAAAAAAAAAAAAAAAAAA", "/assets/index-test123.js", "/does-not-exist"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404 without embedded frontend", path, recorder.Code)
		}
	}
}
