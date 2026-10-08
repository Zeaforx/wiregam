package scripts

import _ "embed"

// Script is an ordered shell script run on a newly created instance through SSM.
// Scripts are embedded into the CLI so deployment never depends on a local path at
// runtime.
type Script struct {
	Name     string
	Contents string
}

//go:embed install-wireguard.sh
var installWireGuardScript string

// Deployment returns the instance configuration steps in deployment order.
func Deployment() []Script {
	return []Script{
		{
			Name:     "install-wireguard",
			Contents: installWireGuardScript,
		},
	}
}
