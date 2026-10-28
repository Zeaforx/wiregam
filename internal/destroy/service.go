package destroy

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/providers/aws"
)

// Destroyer handles the destruction flow for one cloud provider.
type Service struct {
	destroyers map[string]Destroyer
}

func NewService(destroyers ...Destroyer) Service {
	registered := make(map[string]Destroyer, len(destroyers))
	for _, destroyer := range destroyers {
		registered[destroyer.ProviderID()] = destroyer
	}

	return Service{destroyers: registered}
}

// DefaultService contains the destruction flows currently supported by WireGam.
func DefaultService() Service {
	return NewService(aws.Destroyer{})
}

func (s Service) Destroy(ctx context.Context, providerID string) error {
	destroyer, found := s.destroyers[providerID]
	if !found {
		return fmt.Errorf("destroy is not implemented for provider %q", providerID)
	}

	return destroyer.Destroy(ctx)
}
