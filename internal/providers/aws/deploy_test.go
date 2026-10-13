package aws

import "testing"

func TestServerPublicKeyFromOutput(t *testing.T) {
	const want = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	output := "WireGuard is active.\nWIREGAM_SERVER_PUBLIC_KEY=" + want + "\n"
	got, err := serverPublicKeyFromOutput(output)
	if err != nil {
		t.Fatalf("serverPublicKeyFromOutput() error = %v", err)
	}
	if got != want {
		t.Fatalf("serverPublicKeyFromOutput() = %q; want %q", got, want)
	}
}

func TestServerPublicKeyFromOutputRejectsMissingOrInvalidKey(t *testing.T) {
	for _, output := range []string{
		"WireGuard is active.\n",
		"WIREGAM_SERVER_PUBLIC_KEY=not-a-key\n",
	} {
		if _, err := serverPublicKeyFromOutput(output); err == nil {
			t.Fatalf("serverPublicKeyFromOutput(%q) succeeded", output)
		}
	}
}
