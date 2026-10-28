package destroy

import "context"

type Destroyer interface {
	ProviderID() string
	Destroy(ctx context.Context) error
}
