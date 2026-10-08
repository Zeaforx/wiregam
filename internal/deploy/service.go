package deploy

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/providers/aws"
)

// Deployer handles the deployment flow for one cloud provider.
type Service struct {
	deployers map[string]Deployer
}

func NewService(deployers ...Deployer) Service {
	registered := make(map[string]Deployer, len(deployers))
	for _, deployer := range deployers {
		registered[deployer.ProviderID()] = deployer
	}

	return Service{deployers: registered}
}

// DefaultService contains the authentication flows currently supported by WireGam.
func DefaultService() Service {
	return NewService(aws.Deployer{})
}

func (s Service) Deploy(ctx context.Context, providerID string) error {
	deployer, found := s.deployers[providerID]
	if !found {
		return fmt.Errorf("deployment is not implemented for provider %q", providerID)
	}

	return deployer.Deploy(ctx)
}
