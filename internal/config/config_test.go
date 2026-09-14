package config

import (
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	absoluteDefault, err := filepath.Abs("./data")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(values map[string]string) LookupEnv {
		return func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		}
	}

	tests := []struct {
		name           string
		args           []string
		env            map[string]string
		wantAddr       string
		wantFileAddr   string
		wantFileOrigin string
		wantDir        string
	}{
		{"defaults", nil, nil, "127.0.0.1:8080", "127.0.0.1:8081", "http://127.0.0.1:8081", absoluteDefault},
		{"environment", nil, map[string]string{"UPASTE_ADDR": "localhost:9000", "UPASTE_FILE_ADDR": "localhost:9001", "UPASTE_FILE_ORIGIN": "https://files.example.com/", "UPASTE_DATA_DIR": "env-data"}, "localhost:9000", "localhost:9001", "https://files.example.com", "env-data"},
		{"CLI", []string{"-addr", "0.0.0.0:7000", "-file-addr", "0.0.0.0:7001", "-file-origin", "http://files.test:7001/", "-data-dir", "cli-data"}, nil, "0.0.0.0:7000", "0.0.0.0:7001", "http://files.test:7001", "cli-data"},
		{"CLI over environment", []string{"-addr", "127.0.0.1:6000", "-file-addr", "127.0.0.1:6001", "-file-origin", "https://cli.example"}, map[string]string{"UPASTE_ADDR": "localhost:9000", "UPASTE_FILE_ADDR": "localhost:9001", "UPASTE_FILE_ORIGIN": "https://env.example", "UPASTE_DATA_DIR": "env-data"}, "127.0.0.1:6000", "127.0.0.1:6001", "https://cli.example", "env-data"},
		{"CLI over invalid environment", []string{"-addr", "127.0.0.1:6000", "-file-addr", "127.0.0.1:6001", "-file-origin", "https://cli.example", "-data-dir", "cli-data"}, map[string]string{"UPASTE_ADDR": "", "UPASTE_FILE_ADDR": "", "UPASTE_FILE_ORIGIN": "", "UPASTE_DATA_DIR": ""}, "127.0.0.1:6000", "127.0.0.1:6001", "https://cli.example", "cli-data"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Parse(test.args, lookup(test.env))
			if err != nil {
				t.Fatal(err)
			}
			wantDir := test.wantDir
			if !filepath.IsAbs(wantDir) {
				wantDir, err = filepath.Abs(wantDir)
				if err != nil {
					t.Fatal(err)
				}
			}
			if got.Addr != test.wantAddr || got.FileAddr != test.wantFileAddr || got.FileOrigin != test.wantFileOrigin || got.DataDir != wantDir {
				t.Errorf("Parse() = %+v", got)
			}
		})
	}
}

func TestParseTrustedProxyCIDRs(t *testing.T) {
	config, err := Parse([]string{"-trusted-proxy-cidrs", "127.0.0.1/32, ::1/128"}, nil)
	if err != nil || len(config.TrustedProxies) != 2 || config.TrustedProxies[0].String() != "127.0.0.1/32" {
		t.Fatalf("config/error = %+v/%v", config, err)
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"empty environment address", nil, map[string]string{"UPASTE_ADDR": ""}},
		{"empty environment data directory", nil, map[string]string{"UPASTE_DATA_DIR": " "}},
		{"empty CLI address", []string{"-addr="}, nil},
		{"missing port", []string{"-addr", "localhost"}, nil},
		{"empty host", []string{"-addr", ":8080"}, nil},
		{"invalid port", []string{"-addr", "localhost:70000"}, nil},
		{"empty CLI data directory", []string{"-data-dir="}, nil},
		{"identical listeners", []string{"-file-addr", "127.0.0.1:8080"}, nil},
		{"invalid file address", []string{"-file-addr", "localhost"}, nil},
		{"origin scheme", []string{"-file-origin", "ftp://files.example"}, nil},
		{"origin missing host", []string{"-file-origin", "https:///files"}, nil},
		{"origin userinfo", []string{"-file-origin", "https://user@files.example"}, nil},
		{"origin path", []string{"-file-origin", "https://files.example/path"}, nil},
		{"origin query", []string{"-file-origin", "https://files.example?q=x"}, nil},
		{"origin fragment", []string{"-file-origin", "https://files.example/#x"}, nil},
		{"invalid trusted proxy CIDR", []string{"-trusted-proxy-cidrs", "not-a-cidr"}, nil},
		{"unknown flag", []string{"-unknown"}, nil},
		{"positional argument", []string{"unexpected"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(name string) (string, bool) {
				value, ok := test.env[name]
				return value, ok
			}
			if _, err := Parse(test.args, lookup); err == nil {
				t.Fatal("Parse unexpectedly succeeded")
			}
		})
	}
}
