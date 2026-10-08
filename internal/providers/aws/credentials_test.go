package aws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := Credentials{AccessKey: "test-access-key", SecretKey: "test-secret-key"}
	if err := StoreCredentials(want); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}

	got, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if got != want {
		t.Fatalf("LoadCredentials() = %#v, want %#v", got, want)
	}
}

func TestLoadCredentialsMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := LoadCredentials()
	if err == nil || !strings.Contains(err.Error(), "wiregam auth --provider aws") {
		t.Fatalf("LoadCredentials() error = %v, want auth guidance", err)
	}
}

func TestLoadCredentialsRejectsIncompleteFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	credentialsDir := filepath.Join(homeDir, ".wiregam", "creds", ID)
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credentialsDir, "aws.json"), []byte(`{"access_key":"test-access-key"}`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadCredentials()
	if err == nil || !strings.Contains(err.Error(), "missing access_key or secret_key") {
		t.Fatalf("LoadCredentials() error = %v, want missing-key error", err)
	}
}
