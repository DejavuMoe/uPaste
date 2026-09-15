package httpapi

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestAbandonedUploadsLeaveNoSharesOrFinalObjects sends real partial multipart
// HTTP requests, closes the connection before completion, and proves the
// handler cleans staging, commits no Share, and returns to a bounded goroutine
// baseline while the service remains usable.
func TestAbandonedUploadsLeaveNoSharesOrFinalObjects(t *testing.T) {
	env := newAPITestEnv(t)
	server := httptest.NewServer(env.handler)
	defer server.Close()

	runtime.GC()
	baseline := runtime.NumGoroutine()

	const boundary = "phase10abandonedboundary"
	bodyPrefix := "--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"metadata\"\r\n" +
		"Content-Type: application/json\r\n\r\n" +
		`{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}` + "\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Disposition: form-data; name=\"file\"; filename=\"abandoned.bin\"\r\n" +
		"Content-Type: application/octet-stream\r\n\r\n"

	for range 25 {
		connection, err := net.DialTimeout("tcp", server.Listener.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		headers := fmt.Sprintf(
			"POST /api/v1/shares HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\n\r\n",
			server.Listener.Addr().String(), boundary, 8<<20,
		)
		if _, err := connection.Write([]byte(headers + bodyPrefix)); err != nil {
			t.Fatal(err)
		}
		if _, err := connection.Write([]byte(strings.Repeat("a", 64<<10))); err != nil {
			t.Fatal(err)
		}
		_ = connection.Close()
	}

	// Wait for DB state and object staging cleanup to settle.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var shares, payloads int
		if err := env.db.QueryRow("SELECT count(*) FROM shares").Scan(&shares); err != nil {
			t.Fatal(err)
		}
		if err := env.db.QueryRow("SELECT count(*) FROM file_payloads").Scan(&payloads); err != nil {
			t.Fatal(err)
		}
		finalObjects := countFinalObjects(t, filepath.Join(env.dataDir, "objects"))
		if shares == 0 && payloads == 0 && finalObjects == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	var shares, payloads int
	if err := env.db.QueryRow("SELECT count(*) FROM shares").Scan(&shares); err != nil {
		t.Fatal(err)
	}
	if err := env.db.QueryRow("SELECT count(*) FROM file_payloads").Scan(&payloads); err != nil {
		t.Fatal(err)
	}
	if shares != 0 || payloads != 0 {
		t.Fatalf("abandoned uploads committed rows: shares=%d payloads=%d", shares, payloads)
	}
	if final := countFinalObjects(t, filepath.Join(env.dataDir, "objects")); final != 0 {
		t.Fatalf("abandoned uploads left %d final object files", final)
	}

	// Goroutines must return to a bounded baseline once the server closes.
	server.Close()
	for range 100 {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+15 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > baseline+20 {
		t.Fatalf("goroutines did not return to baseline: before=%d after=%d", baseline, after)
	}

	// The service must still accept a normal upload after the aborted ones.
	response := env.serve(multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "after-abandon.bin", []byte("ok"), false))
	if response.Code != http.StatusCreated {
		t.Fatalf("upload after abandoned requests status = %d; body=%s", response.Code, response.Body.String())
	}
}

func countFinalObjects(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".stage-") {
			return nil
		}
		count++
		return nil
	})
	if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatal(err)
	}
	return count
}
