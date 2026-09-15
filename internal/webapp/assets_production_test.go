//go:build production

package webapp

import (
	"io/fs"
	"testing"
)

// TestEmbeddedProductionBundle proves that a -tags production build actually
// contains a staged Vite bundle rather than compiling against an empty tree.
func TestEmbeddedProductionBundle(t *testing.T) {
	handler, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded() error = %v", err)
	}
	if handler == nil {
		t.Fatal("NewEmbedded() = nil, want production handler")
	}
	if _, err := fs.Stat(handler.assets, indexFile); err != nil {
		t.Fatalf("embedded index missing: %v", err)
	}
	entries, err := fs.ReadDir(handler.assets, "assets")
	if err != nil {
		t.Fatalf("embedded assets directory missing: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("embedded assets directory is empty")
	}
}
