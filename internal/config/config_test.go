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
		name     string
		args     []string
		env      map[string]string
		wantAddr string
		wantDir  string
	}{
		{"defaults", nil, nil, "127.0.0.1:8080", absoluteDefault},
		{"environment", nil, map[string]string{"UPASTE_ADDR": "localhost:9000", "UPASTE_DATA_DIR": "env-data"}, "localhost:9000", "env-data"},
		{"CLI", []string{"-addr", "0.0.0.0:7000", "-data-dir", "cli-data"}, nil, "0.0.0.0:7000", "cli-data"},
		{"CLI over environment", []string{"-addr", "127.0.0.1:6000", "-data-dir", "cli-data"}, map[string]string{"UPASTE_ADDR": "localhost:9000", "UPASTE_DATA_DIR": "env-data"}, "127.0.0.1:6000", "cli-data"},
		{"CLI over invalid environment", []string{"-addr", "127.0.0.1:6000", "-data-dir", "cli-data"}, map[string]string{"UPASTE_ADDR": "", "UPASTE_DATA_DIR": ""}, "127.0.0.1:6000", "cli-data"},
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
			if got.Addr != test.wantAddr || got.DataDir != wantDir {
				t.Errorf("Parse() = %+v, want Addr=%q DataDir=%q", got, test.wantAddr, wantDir)
			}
		})
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
