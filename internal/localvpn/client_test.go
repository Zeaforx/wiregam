package localvpn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func TestRenderConfig(t *testing.T) {
	config := renderConfig(testKey, testKey, "203.0.113.10")
	for _, want := range []string{
		"PrivateKey = " + testKey,
		"Address = 10.8.0.2/24",
		"PublicKey = " + testKey,
		"Endpoint = 203.0.113.10:51820",
		"AllowedIPs = 0.0.0.0/0",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("renderConfig() missing %q", want)
		}
	}
}

func TestValidateKey(t *testing.T) {
	if err := validateKey(testKey); err != nil {
		t.Fatalf("validateKey(valid) error = %v", err)
	}
	for _, invalid := range []string{"", "not-base64", testKey + "\n"} {
		if err := validateKey(invalid); err == nil {
			t.Errorf("validateKey(%q) succeeded", invalid)
		}
	}
}

func TestWriteAtomicSetsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := writeAtomic(path, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions = %o; want 600", got)
	}
}
