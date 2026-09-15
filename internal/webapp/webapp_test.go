package webapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testBundle() fstest.MapFS {
	return fstest.MapFS{
		"index.html":              &fstest.MapFile{Data: []byte("<!doctype html><html><body><div id=\"root\"></div></body></html>")},
		"assets/index-abc123.js":  &fstest.MapFile{Data: []byte("console.log('ok')")},
		"assets/index-abc123.css": &fstest.MapFile{Data: []byte("body{margin:0}")},
		"favicon.svg":             &fstest.MapFile{Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>")},
	}
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	handler, err := New(testBundle())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return handler
}

func TestSPAEntryRoutesServeIndex(t *testing.T) {
	handler := newTestHandler(t)
	routes := []string{"/", "/index.html", "/s/AAAAAAAAAAAAAAAAAAAAAA", "/manage/this-is-a-share-id"}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
			if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
			if !strings.Contains(recorder.Body.String(), `id="root"`) {
				t.Fatalf("index body missing root element: %q", recorder.Body.String())
			}
		})
	}
}

func TestUnknownRoutesRemainNotFound(t *testing.T) {
	handler := newTestHandler(t)
	routes := []string{
		"/does-not-exist",
		"/admin",
		"/login",
		"/dashboard",
		"/s/",
		"/s/a/b",
		"/manage/",
		"/f/AAAAAAAAAAAAAAAAAAAAAA",
		"/assets/missing-abc123.js",
		"/api/v1/shares/AAAAAAAAAAAAAAAAAAAAAA",
		"/raw/AAAAAAAAAAAAAAAAAAAAAA",
	}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", recorder.Code)
			}
			if strings.Contains(recorder.Body.String(), `id="root"`) {
				t.Fatalf("unknown route served the SPA index")
			}
		})
	}
}

func TestAssetsHaveExpectedCacheAndContentType(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		path        string
		contentType string
		cache       string
	}{
		{"/assets/index-abc123.js", "text/javascript; charset=utf-8", immutableCache},
		{"/assets/index-abc123.css", "text/css; charset=utf-8", immutableCache},
		{"/favicon.svg", "image/svg+xml", htmlCache},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, test.contentType)
			}
			if got := recorder.Header().Get("Cache-Control"); got != test.cache {
				t.Fatalf("Cache-Control = %q, want %q", got, test.cache)
			}
		})
	}
}

func TestFrontendSecurityHeaders(t *testing.T) {
	handler := newTestHandler(t)
	for _, route := range []string{"/", "/assets/index-abc123.js", "/does-not-exist"} {
		t.Run(route, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
			header := recorder.Header()
			if got := header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := header.Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("Referrer-Policy = %q", got)
			}
			if got := header.Get("X-Frame-Options"); got != "DENY" {
				t.Fatalf("X-Frame-Options = %q", got)
			}
			csp := header.Get("Content-Security-Policy")
			if csp != contentSecurityPolicy {
				t.Fatalf("Content-Security-Policy = %q", csp)
			}
			if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "*") {
				t.Fatalf("CSP is not strict: %q", csp)
			}
		})
	}
}

func TestFrontendMethods(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodPost, "/", http.StatusMethodNotAllowed},
		{http.MethodPost, "/s/AAAAAAAAAAAAAAAAAAAAAA", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/manage/AAAAAAAAAAAAAAAAAAAAAA", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/assets/index-abc123.js", http.StatusMethodNotAllowed},
		{http.MethodPost, "/does-not-exist", http.StatusNotFound},
		{http.MethodPost, "/assets/missing-abc123.js", http.StatusNotFound},
		{http.MethodPost, "/f/AAAAAAAAAAAAAAAAAAAAAA", http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if test.status == http.StatusMethodNotAllowed {
				if got := recorder.Header().Get("Allow"); got != "GET, HEAD" {
					t.Fatalf("Allow = %q, want GET, HEAD", got)
				}
			}
		})
	}
}

func TestHeadRequestsHaveNoBody(t *testing.T) {
	handler := newTestHandler(t)
	for _, route := range []string{"/", "/assets/index-abc123.js"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodHead, route, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", route, recorder.Code)
		}
		if recorder.Body.Len() != 0 {
			t.Fatalf("%s body length = %d, want 0", route, recorder.Body.Len())
		}
	}
}

func TestTraversalIsRejected(t *testing.T) {
	handler := newTestHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.URL.Path = "/../index.html"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestNewRejectsInvalidBundles(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) error = nil, want error")
	}
	if _, err := New(fstest.MapFS{}); err == nil {
		t.Fatal("New(empty) error = nil, want error")
	}
	if _, err := New(fstest.MapFS{"index.html": &fstest.MapFile{Data: nil}}); err == nil {
		t.Fatal("New(empty index) error = nil, want error")
	}
}
