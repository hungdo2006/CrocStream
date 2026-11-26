package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRelayFileMissing(t *testing.T) {
	opts, err := LoadRelayFile(filepath.Join(t.TempDir(), "relay.json"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if opts != (Options{}) {
		t.Fatalf("expected empty options for missing file, got %+v", opts)
	}
}

func TestLoadRelayFileValidAndUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relay.json")
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("{\"address\":\" 198.51.100.10:9009 \",\"password\":\"example-relay-pass\"}\n")...)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	opts, err := LoadRelayFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Address != "198.51.100.10:9009" || opts.Password != "example-relay-pass" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestLoadRelayFileRejectsMalformedInputs(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid-json", "{not json"},
		{"unknown-field", `{"address":"127.0.0.1:9009","password":"secret-password","extra":1}`},
		{"trailing-data", `{"address":"127.0.0.1:9009","password":"secret-password"}{}`},
		{"missing-port", `{"address":"127.0.0.1","password":"secret-password"}`},
		{"empty-password", `{"address":"127.0.0.1:9009","password":""}`},
		{"oversized", `{"address":"127.0.0.1:9009","password":"` + strings.Repeat("a", maxRelayConfigBytes) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "relay.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			opts, err := LoadRelayFile(path)
			if err == nil {
				t.Fatalf("expected error for %s, got options %+v", tc.name, opts)
			}
			if opts != (Options{}) {
				t.Fatalf("expected empty options on error, got %+v", opts)
			}
		})
	}
}
