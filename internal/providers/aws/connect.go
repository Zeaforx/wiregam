package aws

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/localvpn"
)

type Connecter struct{}

func (Connecter) ProviderID() string {
	return ID
}

func (Connecter) Connect(ctx context.Context) error {
	if err := localvpn.Connect(ctx); err != nil {
		return fmt.Errorf("connect WireGuard tunnel: %w", err)
	}
	return nil
}
