package disconnect

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/providers/aws"
)

type Service struct {
	disconnecter map[string]Disconnecter
}

func NewService(disconnecters ...Disconnecter) Service {
	registered := make(map[string]Disconnecter, len(disconnecters))
	for _, disconnecter := range disconnecters {
		registered[disconnecter.ProviderID()] = disconnecter
	}

	return Service{disconnecter: registered}
}

func DefaultService() Service {
	return NewService(aws.Disconnecter{})
}

func (s Service) Disconnect(ctx context.Context, providerID string) error {
	disconnecter, found := s.disconnecter[providerID]
	if !found {
		return fmt.Errorf("disconnection is not implemented for provider %q", providerID)
	}

	return disconnecter.Disconnect(ctx)
}
