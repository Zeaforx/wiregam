package aws

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/zeaforx/wiregam/internal/localvpn"
)

const destroyBatchSize = 1000

type Destroyer struct{}

type destroyEC2Client interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput, ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
}

type confirmDestroy func(instanceIDs []string, region string) (bool, error)
type disconnectLocalVPN func(context.Context) (bool, error)

func (Destroyer) ProviderID() string {
	return ID
}

func (Destroyer) Destroy(ctx context.Context) error {
	storedCredentials, err := LoadCredentials()
	if err != nil {
		return err
	}

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			storedCredentials.AccessKey,
			storedCredentials.SecretKey,
			"",
		)),
	)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	selectedRegion, err := SelectAWSRegion(ctx, cfg)
	if err != nil {
		return err
	}
	cfg.Region = selectedRegion

	return destroyInstances(ctx, ec2.NewFromConfig(cfg), selectedRegion, askToDestroy, localvpn.DisconnectIfActive)
}

func destroyInstances(
	ctx context.Context,
	client destroyEC2Client,
	region string,
	confirm confirmDestroy,
	disconnect disconnectLocalVPN,
) error {
	instanceIDs, err := findWireGamInstances(ctx, client)
	if err != nil {
		return err
	}

	if len(instanceIDs) == 0 {
		fmt.Printf("No WireGam instances found in %s.\n", region)
		return nil
	}

	fmt.Printf("Found %d WireGam instance(s) in %s: %v\n", len(instanceIDs), region, instanceIDs)
	confirmed, err := confirm(instanceIDs, region)
	if err != nil {
		return fmt.Errorf("confirm instance termination: %w", err)
	}
	if !confirmed {
		fmt.Println("Destroy cancelled; no instances were terminated.")
		return nil
	}

	disconnected, err := disconnect(ctx)
	if err != nil {
		return fmt.Errorf("disconnect active WireGuard tunnel before termination: %w", err)
	}
	if disconnected {
		fmt.Println("Disconnected the active WireGuard tunnel before termination.")
	}

	for start := 0; start < len(instanceIDs); start += destroyBatchSize {
		end := min(start+destroyBatchSize, len(instanceIDs))
		result, err := client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs[start:end],
		})
		if err != nil {
			return fmt.Errorf("terminate WireGam instances in %s: %w", region, err)
		}

		for _, instance := range result.TerminatingInstances {
			fmt.Printf(
				"Instance %s: state changed from %q to %q\n",
				aws.ToString(instance.InstanceId),
				instanceStateName(instance.PreviousState),
				instanceStateName(instance.CurrentState),
			)
		}
	}

	return nil
}

func findWireGamInstances(ctx context.Context, client destroyEC2Client) ([]string, error) {
	filters := []types.Filter{
		{
			Name:   aws.String("tag:Project"),
			Values: []string{"wiregam-instance"},
		},
		{
			Name:   aws.String("instance-state-name"),
			Values: []string{"pending", "running", "stopping", "stopped"},
		},
	}

	var instanceIDs []string
	var nextToken *string
	for {
		result, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters:   filters,
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("find WireGam instances: %w", err)
		}

		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				if instance.InstanceId != nil {
					instanceIDs = append(instanceIDs, *instance.InstanceId)
				}
			}
		}

		if result.NextToken == nil || *result.NextToken == "" {
			break
		}
		nextToken = result.NextToken
	}

	return instanceIDs, nil
}

func askToDestroy(instanceIDs []string, region string) (bool, error) {
	confirmed := false
	err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf(
			"Permanently terminate %d WireGam instance(s) in %s?",
			len(instanceIDs),
			region,
		),
		Default: false,
	}, &confirmed)
	return confirmed, err
}

func instanceStateName(state *types.InstanceState) string {
	if state == nil {
		return "unknown"
	}
	return string(state.Name)
}
