package fileapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
)

type testEnv struct {
	db      *sql.DB
	store   *objectstore.Local
	service *share.Service
	handler http.Handler
	now     *time.Time
	log     *slog.Logger
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := objectstore.OpenLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	service := share.NewWithStore(db, store, func() time.Time { return now })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := &testEnv{db: db, store: store, service: service, now: &now, log: log, handler: New(service, log)}
	t.Cleanup(func() { _ = db.Close() })
	return env
}

func (env *testEnv) create(t *testing.T, filename, content string, expiration *time.Time) (share.Share, string) {
	t.Helper()
	staged, err := env.service.StageFile(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	value, token, err := env.service.CreateFile(context.Background(), filename, staged, expiration)
	if err != nil {
		t.Fatal(err)
	}
	return value, token.Reveal()
}

func (env *testEnv) request(method, path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	env.handler.ServeHTTP(response, request)
	return response
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	writeDeadlines []time.Time
}

func (recorder *deadlineRecorder) SetWriteDeadline(value time.Time) error {
	recorder.writeDeadlines = append(recorder.writeDeadlines, value)
	return nil
}

func TestFileResponseDeadlineUnwrapsSecurityAndHEADWriters(t *testing.T) {
	env := newTestEnv(t)
	value, _ := env.create(t, "x.txt", "x", nil)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			response := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			env.handler.ServeHTTP(response, httptest.NewRequest(method, "/f/"+value.ID.String(), nil))
			if response.Code != http.StatusOK || len(response.writeDeadlines) != 1 || time.Until(response.writeDeadlines[0]) < share.FileTransferDeadline-time.Second {
				t.Fatalf("deadline/status = %v/%d", response.writeDeadlines, response.Code)
			}
			if method == http.MethodHead && response.Body.Len() != 0 {
				t.Fatal("HEAD had body")
			}
		})
	}
}

func TestFileDownloadDeadlineReachesRealServerConnection(t *testing.T) {
	env := newTestEnv(t)
	value, _ := env.create(t, "x.txt", "x", nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tracked := &deadlineListener{Listener: listener, connections: make(chan *deadlineConn, 1)}
	server := &http.Server{Handler: env.handler, WriteTimeout: 15 * time.Second}
	go server.Serve(tracked)
	t.Cleanup(func() { _ = server.Close() })
	response, err := http.Get("http://" + listener.Addr().String() + "/f/" + value.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	connection := <-tracked.connections
	if !connection.hasWriteDeadlineAfter(time.Now().Add(9 * time.Minute)) {
		t.Fatal("File download deadline did not reach real connection")
	}
}

func TestFileDelivery(t *testing.T) {
	env := newTestEnv(t)
	value, _ := env.create(t, `quote".txt`, "hello file", nil)
	path := "/f/" + value.ID.String()
	response := env.request(http.MethodGet, path, nil)
	if response.Code != http.StatusOK || response.Body.String() != "hello file" {
		t.Fatalf("GET = %d/%q", response.Code, response.Body.String())
	}
	assertHeaders(t, response, value)
	if got := response.Header().Get("Content-Length"); got != "10" {
		t.Fatalf("Content-Length = %q", got)
	}
	if got := response.Header().Get("ETag"); got != `"`+sha256Hex([]byte("hello file"))+`"` {
		t.Fatalf("ETag = %q", got)
	}
	if strings.Contains(response.Header().Get("Content-Disposition"), "\r") || strings.Contains(response.Header().Get("Content-Disposition"), "\n") {
		t.Fatal("header injection")
	}

	head := env.request(http.MethodHead, path, nil)
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD = %d/%d", head.Code, head.Body.Len())
	}
	assertHeaders(t, head, value)

	rangeResponse := env.request(http.MethodGet, path, map[string]string{"Range": "bytes=1-4"})
	if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.String() != "ello" {
		t.Fatalf("range = %d/%q", rangeResponse.Code, rangeResponse.Body.String())
	}
	assertHeaders(t, rangeResponse, value)
	invalidRange := env.request(http.MethodGet, path, map[string]string{"Range": "bytes=99-100"})
	if invalidRange.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("invalid range = %d", invalidRange.Code)
	}
	assertHeaders(t, invalidRange, value)
}

func TestFileMIMESniffingAlwaysUsesAttachment(t *testing.T) {
	env := newTestEnv(t)
	for _, content := range []string{"<html>", "<script>", "<svg", "%PDF-", "\x89PNG\r\n\x1a\n", "plain UTF-8 text", "\x00\x01\x02"} {
		value, _ := env.create(t, "unsafe.html", content, nil)
		response := env.request(http.MethodGet, "/f/"+value.ID.String(), nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d", response.Code)
		}
		if response.Header().Get("Content-Type") != http.DetectContentType([]byte(content)) {
			t.Fatalf("MIME = %q", response.Header().Get("Content-Type"))
		}
		if !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") || strings.Contains(response.Header().Get("Content-Disposition"), "inline") {
			t.Fatal("unsafe MIME delivered inline")
		}
	}
}

func TestFileDeliveryLifecycleAndIsolation(t *testing.T) {
	env := newTestEnv(t)
	value, token := env.create(t, "index.html", "<html><script>x</script>", nil)
	path := "/f/" + value.ID.String()
	if got := env.request(http.MethodGet, "/api/v1/shares/"+value.ID.String(), nil).Code; got != http.StatusNotFound {
		t.Fatalf("file listener API route = %d", got)
	}
	if got := env.request(http.MethodGet, "/raw/"+value.ID.String(), nil).Code; got != http.StatusNotFound {
		t.Fatalf("file listener raw route = %d", got)
	}
	response := env.request(http.MethodGet, path, nil)
	if response.Header().Get("Content-Disposition") == "inline" || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("HTML was not attachment")
	}

	expires := env.now.Add(time.Hour)
	expiring, _ := env.create(t, "expired.bin", "x", &expires)
	*env.now = expires
	if response := env.request(http.MethodGet, "/f/"+expiring.ID.String(), nil); response.Code != http.StatusGone {
		t.Fatalf("expired = %d", response.Code)
	}
	assertSecurity(t, env.request(http.MethodGet, "/f/"+expiring.ID.String(), nil))
	*env.now = expires.Add(-time.Minute)
	if err := env.service.Delete(context.Background(), value.ID, token); err != nil {
		t.Fatal(err)
	}
	if response := env.request(http.MethodGet, path, nil); response.Code != http.StatusNotFound {
		t.Fatalf("deleted = %d", response.Code)
	}

	text, _, err := env.service.Create(context.Background(), share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if response := env.request(http.MethodGet, "/f/"+text.ID.String(), nil); response.Code != http.StatusNotFound {
		t.Fatalf("Text through file listener = %d", response.Code)
	}
	missingHead := env.request(http.MethodHead, "/f/not-an-id", nil)
	if missingHead.Code != http.StatusNotFound || missingHead.Body.Len() != 0 {
		t.Fatalf("HEAD missing = %d, body=%q", missingHead.Code, missingHead.Body.String())
	}
	assertSecurity(t, missingHead)
}

func TestMissingObjectIsInternalError(t *testing.T) {
	env := newTestEnv(t)
	value, _ := env.create(t, "missing.bin", "x", nil)
	if err := env.store.Delete(value.File.StorageKey); err != nil {
		t.Fatal(err)
	}
	response := env.request(http.MethodGet, "/f/"+value.ID.String(), nil)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("missing object = %d", response.Code)
	}
	assertSecurity(t, response)
}

func assertHeaders(t *testing.T, response *httptest.ResponseRecorder, value share.Share) {
	t.Helper()
	assertSecurity(t, response)
	if got := response.Header().Get("Content-Type"); got != value.File.MediaType {
		t.Fatalf("Content-Type = %q, want %q", got, value.File.MediaType)
	}
	if !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("missing attachment disposition")
	}
}
func assertSecurity(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	for name, want := range map[string]string{"X-Content-Type-Options": "nosniff", "Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY", "Content-Security-Policy": "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox"} {
		if response.Header().Get(name) != want {
			t.Errorf("%s = %q", name, response.Header().Get(name))
		}
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS header set")
	}
}

type deadlineListener struct {
	net.Listener
	connections chan *deadlineConn
}

func (listener *deadlineListener) Accept() (net.Conn, error) {
	connection, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tracked := &deadlineConn{Conn: connection}
	listener.connections <- tracked
	return tracked, nil
}

type deadlineConn struct {
	net.Conn
	mu             sync.Mutex
	writeDeadlines []time.Time
}

func (connection *deadlineConn) SetWriteDeadline(value time.Time) error {
	connection.mu.Lock()
	connection.writeDeadlines = append(connection.writeDeadlines, value)
	connection.mu.Unlock()
	return connection.Conn.SetWriteDeadline(value)
}
func (connection *deadlineConn) hasWriteDeadlineAfter(value time.Time) bool {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for _, deadline := range connection.writeDeadlines {
		if deadline.After(value) {
			return true
		}
	}
	return false
}

func sha256Hex(value []byte) string { return fmt.Sprintf("%x", sha256.Sum256(value)) }
