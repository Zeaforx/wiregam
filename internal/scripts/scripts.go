package scripts

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"
)

// Script is an ordered shell script run on a newly created instance through SSM.
// Scripts are embedded into the CLI so deployment never depends on a local path at
// runtime.
type Script struct {
	Name     string
	Contents string
}

//go:embed install-wireguard.sh
var installWireGuardScript string

//go:embed configure-wireguard.sh
var configureWireGuardScript string

//go:embed start-wireguard.sh
var startWireGuardScript string

// Deployment returns the instance configuration steps in deployment order.
// Only the client's public key is placed in the EC2 configuration.
func Deployment(clientPublicKey string) ([]Script, error) {
	if err := validatePublicKey(clientPublicKey); err != nil {
		return nil, fmt.Errorf("client public key: %w", err)
	}

	configuredScript := strings.ReplaceAll(
		configureWireGuardScript,
		"{{CLIENT_PUBLIC_KEY}}",
		clientPublicKey,
	)

	return []Script{
		{
			Name:     "install-wireguard",
			Contents: installWireGuardScript,
		},
		{
			Name:     "configure-wireguard",
			Contents: configuredScript,
		},
		{
			Name:     "start-wireguard",
			Contents: startWireGuardScript,
		},
	}, nil
}

func validatePublicKey(key string) error {
	if strings.TrimSpace(key) != key {
		return fmt.Errorf("contains surrounding whitespace")
	}

	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("must be a base64-encoded 32-byte WireGuard key")
	}

	return nil
}
