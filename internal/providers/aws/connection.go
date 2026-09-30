package aws

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// TestConnection verifies the supplied AWS credentials with STS.
func TestConnection(ctx context.Context, credentialsToTest Credentials) error {
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			credentialsToTest.AccessKey,
			credentialsToTest.SecretKey,
			"",
		)),
	)
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}

	result, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("verify AWS credentials: %w", err)
	}

	fmt.Println("Connected to AWS!")
	fmt.Println("Account:", *result.Account)
	fmt.Println("ARN:", *result.Arn)
	return nil
}
