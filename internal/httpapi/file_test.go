package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func multipartRequest(t *testing.T, metadata string, filename string, content []byte, fileFirst bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writeMetadata := func() {
		header := textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(metadata)); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func() {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if fileFirst {
		writeFile()
		writeMetadata()
	} else {
		writeMetadata()
		writeFile()
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	readDeadlines  []time.Time
	writeDeadlines []time.Time
}

func (recorder *deadlineRecorder) SetReadDeadline(value time.Time) error {
	recorder.readDeadlines = append(recorder.readDeadlines, value)
	return nil
}
func (recorder *deadlineRecorder) SetWriteDeadline(value time.Time) error {
	recorder.writeDeadlines = append(recorder.writeDeadlines, value)
	return nil
}

func TestFileUploadExtendsOnlyMultipartDeadline(t *testing.T) {
	env := newAPITestEnv(t)
	fileResponse := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	env.handler.ServeHTTP(fileResponse, multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "x.txt", []byte("x"), false))
	if fileResponse.Code != http.StatusCreated || len(fileResponse.readDeadlines) != 1 || len(fileResponse.writeDeadlines) != 1 || time.Until(fileResponse.readDeadlines[0]) < share.FileTransferDeadline-time.Second {
		t.Fatalf("File deadline/status = %v/%d", fileResponse.readDeadlines, fileResponse.Code)
	}
	jsonResponse := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x"},"expires_at":null}`))
	request.Header.Set("Content-Type", "application/json")
	env.handler.ServeHTTP(jsonResponse, request)
	if jsonResponse.Code != http.StatusCreated || len(jsonResponse.readDeadlines) != 0 || len(jsonResponse.writeDeadlines) != 0 {
		t.Fatalf("JSON deadline/status = %v/%d", jsonResponse.readDeadlines, jsonResponse.Code)
	}
}

func TestFileUploadDeadlineReachesRealServerConnection(t *testing.T) {
	env := newAPITestEnv(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tracked := &deadlineListener{Listener: listener, connections: make(chan *deadlineConn, 1)}
	server := &http.Server{Handler: env.handler, ReadTimeout: 15 * time.Second}
	go server.Serve(tracked)
	t.Cleanup(func() { _ = server.Close() })
	request := multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "x.txt", []byte("x"), false)
	request.URL.Scheme, request.URL.Host, request.RequestURI = "http", listener.Addr().String(), ""
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", response.StatusCode)
	}
	connection := <-tracked.connections
	if !connection.hasReadDeadlineAfter(time.Now().Add(9*time.Minute)) || !connection.hasWriteDeadlineAfter(time.Now().Add(9*time.Minute)) {
		t.Fatal("File upload deadlines did not reach real connection")
	}
}

func TestCreateFileShare(t *testing.T) {
	env := newAPITestEnv(t)
	metadata := `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`
	for _, fileFirst := range []bool{false, true} {
		response := env.serve(multipartRequest(t, metadata, `C:\fakepath\report.pdf`, []byte("%PDF-test"), fileFirst))
		if response.Code != http.StatusCreated {
			t.Fatalf("create file status = %d; body=%s", response.Code, response.Body.String())
		}
		var created createResponse
		if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.Share.PayloadKind != "FILE" || created.Share.PrivacyMode != "STANDARD" || created.Share.File == nil || created.Share.Text != nil || created.Share.EncryptedText != nil {
			t.Fatal("file response shape is incorrect")
		}
		if created.Share.File.Filename != "report.pdf" || created.Share.File.Size != 9 || created.Share.File.DownloadURL != "https://files.example.test/f/"+created.Share.ID || len(created.Share.File.SHA256) != 64 {
			t.Fatalf("file response = %+v", created.Share.File)
		}
		if strings.Contains(response.Body.String(), "storage_key") || strings.Contains(response.Body.String(), "objects/") {
			t.Fatal("file response exposed internal storage")
		}
		var key, name, media string
		var size int64
		if err := env.db.QueryRow("SELECT storage_key, original_filename, size_bytes, detected_media_type FROM file_payloads WHERE share_id = ?", created.Share.ID).Scan(&key, &name, &size, &media); err != nil {
			t.Fatal(err)
		}
		if len(key) != 32 || name != "report.pdf" || size != 9 || media != "application/pdf" {
			t.Fatalf("stored file metadata = %q/%q/%d/%q", key, name, size, media)
		}
		if _, err := capability.ParseOwnerToken(created.OwnerToken); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFileMultipartValidation(t *testing.T) {
	env := newAPITestEnv(t)
	valid := `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`
	// Construct malformed shapes directly to keep the test exact.
	cases := []struct {
		name  string
		build func(*multipart.Writer)
	}{
		{"missing metadata", func(w *multipart.Writer) { p, _ := w.CreateFormFile("file", "x"); _, _ = p.Write([]byte("x")) }},
		{"missing file", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
		}},
		{"duplicate metadata", func(w *multipart.Writer) {
			for range 2 {
				p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
				_, _ = p.Write([]byte(valid))
			}
			p, _ := w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"duplicate file", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
			for range 2 {
				p, _ := w.CreateFormFile("file", "x")
				_, _ = p.Write([]byte("x"))
			}
		}},
		{"unknown part", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
			p, _ = w.CreateFormField("other")
			_, _ = p.Write([]byte("x"))
		}},
		{"missing filename", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
			p, _ = w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="file"`}})
			_, _ = p.Write([]byte("x"))
		}},
		{"bad metadata media", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"text/plain"}})
			_, _ = p.Write([]byte(valid))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"metadata with filename", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"; filename="x"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"invalid metadata JSON", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(`{"payload_kind":`))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"zero file", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(valid))
			_, _ = w.CreateFormFile("file", "x")
		}},
		{"encrypted file", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(`{"payload_kind":"FILE","privacy_mode":"ENCRYPTED","expires_at":null}`))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"duplicate metadata JSON", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(`{"payload_kind":"FILE","payload_kind":"TEXT","privacy_mode":"STANDARD","expires_at":null}`))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
		{"unknown metadata JSON", func(w *multipart.Writer) {
			p, _ := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
			_, _ = p.Write([]byte(`{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null,"x":true}`))
			p, _ = w.CreateFormFile("file", "x")
			_, _ = p.Write([]byte("x"))
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			test.build(writer)
			_ = writer.Close()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := env.serve(request)
			status, code := http.StatusBadRequest, "invalid_request"
			if test.name == "encrypted file" {
				status, code = http.StatusUnprocessableEntity, "unsupported_share_type"
			}
			assertError(t, response, status, code)
		})
	}
}

func TestFilenamePolicy(t *testing.T) {
	for filename, want := range map[string]string{
		"../../evil.html":        "evil.html",
		`C:\\fakepath\\evil.svg`: "evil.svg",
		`quote".txt`:             `quote".txt`,
		"semi;colon.txt":         "semi;colon.txt",
		"unicode-报告.pdf":         "unicode-报告.pdf",
	} {
		got, err := sanitizeFilename(filename)
		if err != nil || got != want {
			t.Fatalf("sanitize %q = %q/%v", filename, got, err)
		}
	}
	for _, filename := range []string{"", "\x00x", "line\nbreak", "line\rbreak", strings.Repeat("x", 256), "/", "//", "\\", "\\\\", "..", "../", "a/.."} {
		if _, err := sanitizeFilename(filename); err == nil {
			t.Fatalf("unsafe filename %q accepted", filename)
		}
	}
}

func TestFilePatchAndExpiration(t *testing.T) {
	env := newAPITestEnv(t)
	createdResponse := env.serve(multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "x.txt", []byte("x"), false))
	var created createResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/shares/" + created.Share.ID
	future := env.now.Add(time.Hour)
	if response := authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": future.Format(time.RFC3339Nano)}); response.Code != http.StatusOK {
		t.Fatalf("file expiration patch = %d", response.Code)
	}
	assertError(t, authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"text": map[string]any{"format": "PLAIN", "content": "x"}}), 400, "invalid_request")
	if response := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil); response.Code != http.StatusNotFound {
		t.Fatalf("File raw status = %d", response.Code)
	}
	wrong, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, wrong.Reveal())
	assertError(t, env.serve(request), 401, "unauthorized")
	*env.now = future
	assertError(t, authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": future.Add(time.Hour).Format(time.RFC3339Nano)}), 410, "expired")
}

func TestFileDeleteCascadesObject(t *testing.T) {
	env := newAPITestEnv(t)
	response := env.serve(multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "x.bin", []byte("x"), false))
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var key string
	if err := env.db.QueryRow("SELECT storage_key FROM file_payloads WHERE share_id = ?", created.Share.ID).Scan(&key); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+created.Share.ID, nil)
	bearer(request, created.OwnerToken)
	if response := env.serve(request); response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.Code)
	}
	var count int
	if err := env.db.QueryRow("SELECT count(*) FROM file_payloads WHERE share_id = ?", created.Share.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("file row/error = %d/%v", count, err)
	}
	if _, err := env.store.Open(key); err == nil {
		t.Fatal("deleted object remains")
	}
	if strings.Contains(env.logs.String(), created.OwnerToken) || strings.Contains(env.logs.String(), key) {
		t.Fatal("delete logs leaked secret or key")
	}
	assertError(t, env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil), http.StatusNotFound, "not_found")
}

func TestFileUploadSizeBoundaries(t *testing.T) {
	env := newAPITestEnv(t)
	for _, test := range []struct {
		name   string
		size   int64
		status int
	}{
		{"exact maximum", share.MaxFileBytes, http.StatusCreated},
		{"oversized file", share.MaxFileBytes + 1, http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, writer := io.Pipe()
			multipartWriter := multipart.NewWriter(writer)
			go func() {
				defer writer.Close()
				defer multipartWriter.Close()
				metadata, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="metadata"`}, "Content-Type": {"application/json"}})
				if err == nil {
					_, err = metadata.Write([]byte(`{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`))
				}
				if err == nil {
					var part io.Writer
					part, err = multipartWriter.CreateFormFile("file", "large.bin")
					if err == nil {
						_, err = io.Copy(part, io.LimitReader(zeroReader{}, test.size))
					}
				}
				if err != nil {
					_ = writer.CloseWithError(err)
				}
			}()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", reader)
			request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
			response := env.serve(request)
			if test.status == http.StatusCreated {
				if response.Code != test.status {
					t.Fatalf("status = %d", response.Code)
				}
			} else {
				assertError(t, response, test.status, "request_too_large")
			}
		})
	}
}

func TestFileUploadWireLimit(t *testing.T) {
	env := newAPITestEnv(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", io.LimitReader(zeroReader{}, maxFileWireBytes+1))
	request.ContentLength = maxFileWireBytes + 1
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	assertError(t, env.serve(request), http.StatusRequestEntityTooLarge, "request_too_large")
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
	mu                            sync.Mutex
	readDeadlines, writeDeadlines []time.Time
}

func (connection *deadlineConn) SetReadDeadline(value time.Time) error {
	connection.mu.Lock()
	connection.readDeadlines = append(connection.readDeadlines, value)
	connection.mu.Unlock()
	return connection.Conn.SetReadDeadline(value)
}
func (connection *deadlineConn) SetWriteDeadline(value time.Time) error {
	connection.mu.Lock()
	connection.writeDeadlines = append(connection.writeDeadlines, value)
	connection.mu.Unlock()
	return connection.Conn.SetWriteDeadline(value)
}
func (connection *deadlineConn) hasReadDeadlineAfter(value time.Time) bool {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	for _, deadline := range connection.readDeadlines {
		if deadline.After(value) {
			return true
		}
	}
	return false
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

type zeroReader struct{}

func (zeroReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = 0
	}
	return len(value), nil
}

var _ = share.MaxFileBytes
