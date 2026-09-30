package aws

import (
	"context"
	"fmt"

	"github.com/zeaforx/wiregam/internal/terminal"
)

// Authenticator implements WireGam's AWS credential flow.
type Authenticator struct{}

func (Authenticator) ProviderID() string {
	return ID
}

func (Authenticator) Authenticate(ctx context.Context) error {
	accessKey, err := terminal.ReadInput("Enter AWS access key: ")
	if err != nil {
		return err
	}

	secretKey, err := terminal.ReadSecretInput("Enter AWS secret key: ")
	if err != nil {
		return err
	}

	credentials := Credentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	if err := TestConnection(ctx, credentials); err != nil {
		return err
	}

	if err := StoreCredentials(credentials); err != nil {
		return err
	}

	fmt.Println("AWS credentials stored successfully!")
	return nil
}
