package auth

import "context"

// Authenticator handles the credential flow for one cloud provider.
type Authenticator interface {
	ProviderID() string
	Authenticate(ctx context.Context) error
}
