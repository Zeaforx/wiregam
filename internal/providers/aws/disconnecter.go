package aws

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/localvpn"
)

type Disconnecter struct{}

func (Disconnecter) ProviderID() string {
	return ID
}

func (Disconnecter) Disconnect(ctx context.Context) error {
	if err := localvpn.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnect WireGuard tunnel: %w", err)
	}
	return nil
}
