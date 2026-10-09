package scripts

import (
	"strings"
	"testing"
)

func TestDeploymentOrderAndContents(t *testing.T) {
	wantNames := []string{
		"install-wireguard",
		"configure-wireguard",
		"start-wireguard",
	}

	const clientPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	got, err := Deployment(clientPublicKey)
	if err != nil {
		t.Fatalf("Deployment() error = %v", err)
	}
	if len(got) != len(wantNames) {
		t.Fatalf("Deployment() returned %d scripts; want %d", len(got), len(wantNames))
	}

	for i, wantName := range wantNames {
		if got[i].Name != wantName {
			t.Errorf("Deployment()[%d].Name = %q; want %q", i, got[i].Name, wantName)
		}
		if strings.TrimSpace(got[i].Contents) == "" {
			t.Errorf("Deployment()[%d].Contents is empty", i)
		}
	}

	if !strings.Contains(got[1].Contents, "PublicKey = "+clientPublicKey) {
		t.Error("configure script does not contain the client public key")
	}
	if strings.Contains(got[1].Contents, "{{CLIENT_PUBLIC_KEY}}") {
		t.Error("configure script still contains the client key placeholder")
	}
}

func TestDeploymentRejectsInvalidClientPublicKey(t *testing.T) {
	if _, err := Deployment("not-a-wireguard-key"); err == nil {
		t.Fatal("Deployment() accepted an invalid client public key")
	}
}
