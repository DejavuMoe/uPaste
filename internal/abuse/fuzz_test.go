package abuse

import (
	"net/netip"
	"testing"
)

func fuzzTrustedPrefixes() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8::/32"),
	}
}

// FuzzClientIP proves the core trusted-proxy invariant: an untrusted immediate
// peer can never be replaced by a peer-supplied X-Forwarded-For value.
func FuzzClientIP(f *testing.F) {
	trusted := fuzzTrustedPrefixes()
	for _, seed := range []struct{ peer, xff string }{
		{"198.51.100.2:1", "203.0.113.9"},
		{"127.0.0.1:1", "203.0.113.9"},
		{"127.0.0.1:1", "203.0.113.9, 10.0.0.2"},
		{"127.0.0.1:1", "bad"},
		{"[::1]:1", "2001:db8::5"},
		{"[::ffff:198.51.100.2]:1", ""},
		{"", ""},
	} {
		f.Add(seed.peer, seed.xff)
	}
	f.Fuzz(func(t *testing.T, peer, xff string) {
		parsed, ok := parsePeer(peer)
		if !ok {
			if got := ClientIP(peer, xff, trusted); got.IsValid() {
				t.Fatalf("invalid peer %q resolved to %v", peer, got)
			}
			return
		}
		got := ClientIP(peer, xff, trusted)
		if !isTrusted(parsed, trusted) && got != parsed {
			t.Fatalf("untrusted peer %v was replaced by %v using XFF %q", parsed, got, xff)
		}
	})
}

func FuzzRateKey(f *testing.F) {
	for _, seed := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "2001:db8:1:2::1", "not-an-ip", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		address, err := netip.ParseAddr(value)
		var want string
		switch {
		case err != nil:
			want = "invalid"
		default:
			address = address.Unmap()
			if address.Is4() {
				want = address.String()
			} else {
				want = netip.PrefixFrom(address, 64).Masked().String()
			}
		}
		if got := RateKey(address); got != want {
			t.Fatalf("RateKey(%q) = %q, want %q", value, got, want)
		}
	})
}
