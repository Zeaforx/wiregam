package deploy

import "context"

type Deployer interface {
	ProviderID() string
	Deploy(ctx context.Context) error
}
