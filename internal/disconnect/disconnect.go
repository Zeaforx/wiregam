package disconnect

import "context"

type Disconnecter interface {
	ProviderID() string
	Disconnect(ctx context.Context) error
}
