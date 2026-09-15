package config

import "testing"

func FuzzParseCIDRs(f *testing.F) {
	for _, seed := range []string{
		"",
		"127.0.0.1/32",
		"127.0.0.1/32,::1/128",
		"not-a-cidr",
		"10.0.0.0/8, 192.0.2.0/24",
		"2001:db8::/32",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		prefixes, err := parseCIDRs(value)
		if err != nil {
			return
		}
		for _, prefix := range prefixes {
			if prefix != prefix.Masked() {
				t.Fatalf("parseCIDRs returned unmasked prefix %v", prefix)
			}
		}
	})
}
