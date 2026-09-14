package abuse

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("10.0.0.0/8")}
	for _, test := range []struct{ name, peer, xff, want string }{
		{"untrusted ignores XFF", "198.51.100.2:1", "203.0.113.9", "198.51.100.2"},
		{"trusted uses XFF", "127.0.0.1:1", "203.0.113.9", "203.0.113.9"},
		{"trusted chain", "127.0.0.1:1", "203.0.113.9, 10.0.0.2", "203.0.113.9"},
		{"malformed falls back", "127.0.0.1:1", "bad", "127.0.0.1"},
		{"mapped unmaps", "[::ffff:198.51.100.2]:1", "", "198.51.100.2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ClientIP(test.peer, test.xff, trusted).String(); got != test.want {
				t.Fatalf("IP = %q", got)
			}
		})
	}
	if RateKey(netip.MustParseAddr("2001:db8:1:2::1")) != RateKey(netip.MustParseAddr("2001:db8:1:2::2")) {
		t.Fatal("same /64 differs")
	}
	if RateKey(netip.MustParseAddr("2001:db8:1:2::1")) == RateKey(netip.MustParseAddr("2001:db8:1:3::1")) {
		t.Fatal("different /64 matches")
	}
}

func TestControlLimitsAndBounds(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	control := New(Config{Now: func() time.Time { return now }, Max: 2})
	first := httptest.NewRequest("POST", "/", nil)
	first.RemoteAddr = "198.51.100.1:1"
	if !control.Allow(first, Create) {
		t.Fatal("first request")
	}
	swept := control.lastSweep
	now = now.Add(time.Second)
	if !control.Allow(first, Create) || control.lastSweep != swept {
		t.Fatal("registry swept before interval")
	}
	for count := 0; count < 3; count++ {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "198.51.100.1:1"
		if !control.Allow(r, Create) {
			t.Fatalf("burst denied at %d", count)
		}
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "198.51.100.1:1"
	if control.Allow(r, Create) {
		t.Fatal("burst allowed")
	}
	now = now.Add(6 * time.Second)
	if !control.Allow(r, Create) {
		t.Fatal("refill denied")
	}
	read := httptest.NewRequest("GET", "/", nil)
	read.RemoteAddr = "198.51.100.1:1"
	if !control.Allow(read, Read) {
		t.Fatal("independent read denied")
	}
	for _, ip := range []string{"198.51.100.2", "198.51.100.3"} {
		q := httptest.NewRequest("GET", "/", nil)
		q.RemoteAddr = ip + ":1"
		control.Allow(q, Read)
	}
	if len(control.clients) > 2 {
		t.Fatal("registry exceeded max")
	}
	now = now.Add(ClientIdleTTL)
	q := httptest.NewRequest("GET", "/", nil)
	q.RemoteAddr = "198.51.100.3:1"
	if !control.Allow(q, Read) {
		t.Fatal("idle eviction did not admit")
	}
}
