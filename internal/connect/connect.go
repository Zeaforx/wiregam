package connect

import "context"

type Connecter interface {
	ProviderID() string
	Connect(ctx context.Context) error
}
