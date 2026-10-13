package connect

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/providers/aws"
)

type Service struct {
	connecter map[string]Connecter
}

func NewService(connecters ...Connecter) Service {
	registered := make(map[string]Connecter, len(connecters))
	for _, connecter := range connecters {
		registered[connecter.ProviderID()] = connecter
	}

	return Service{connecter: registered}
}

func DefaultService() Service {
	return NewService(aws.Connecter{})
}

func (s Service) Connect(ctx context.Context, providerID string) error {
	connecter, found := s.connecter[providerID]
	if !found {
		return fmt.Errorf("connection is not implemented for provider %q", providerID)
	}

	return connecter.Connect(ctx)
}
