package auth

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/providers/aws"
)

// Service selects the authentication flow for a provider.
type Service struct {
	authenticators map[string]Authenticator
}

func NewService(authenticators ...Authenticator) Service {
	registered := make(map[string]Authenticator, len(authenticators))
	for _, authenticator := range authenticators {
		registered[authenticator.ProviderID()] = authenticator
	}

	return Service{authenticators: registered}
}

// DefaultService contains the authentication flows currently supported by WireGam.
func DefaultService() Service {
	return NewService(aws.Authenticator{})
}

func (s Service) Authenticate(ctx context.Context, providerID string) error {
	authenticator, found := s.authenticators[providerID]
	if !found {
		return fmt.Errorf("authentication is not implemented for provider %q", providerID)
	}

	return authenticator.Authenticate(ctx)
}
