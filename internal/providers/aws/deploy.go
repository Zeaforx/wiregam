package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go"
	"github.com/zeaforx/wiregam/internal/scripts"
)

type Deployer struct{}

func (Deployer) ProviderID() string {
	return ID
}

func (Deployer) Deploy(ctx context.Context) error {
	// Load stored AWS credentials.
	storedCredentials, err := LoadCredentials()
	if err != nil {
		return err
	}

	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(
			awscredentials.NewStaticCredentialsProvider(
				storedCredentials.AccessKey,
				storedCredentials.SecretKey,
				"",
			),
		),
	)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}

	// 1. Select AWS region.
	selectedRegion, err := SelectAWSRegion(ctx, cfg)
	if err != nil {
		return err
	}

	// Actually configure the AWS SDK to use the selected region.
	cfg.Region = selectedRegion

	fmt.Printf("Selected region: %s\n", selectedRegion)

	// 2. Find latest Amazon Linux 2023 AMI.
	imageID, err := FindLatestAMI(ctx, cfg)
	if err != nil {
		return err
	}

	securityGroupID, err := CreateSecurityGroup(
		ctx,
		cfg,
		securityGroupName,
		"Security group for WireGam VPN",
	)
	if err != nil {
		return err
	}

	localIP, err := getOutboundIP()
	if err != nil {
		return fmt.Errorf("get outbound IP: %w", err)
	}

	log.Printf("Outbound IP: %s\n", localIP.String())

	if err := customizeSecurityGroupPorts(ctx, ec2.NewFromConfig(cfg), securityGroupID, localIP); err != nil {
		return fmt.Errorf("customize security group ports: %w", err)
	}

	// 3. Launch EC2 instance.
	instanceID, err := CreateInstance(ctx, cfg, imageID, securityGroupID)
	if err != nil {
		return err
	}

	// 4. Wait for EC2 to reach running state.
	if err := WaitForInstance(ctx, cfg, instanceID); err != nil {
		return err
	}

	// 5. Wait for the SSM agent to register.
	if err := WaitForSSM(ctx, cfg, instanceID); err != nil {
		return err
	}

	// 6. Configure the instance using the ordered deployment scripts.
	if err := RunDeploymentScripts(ctx, cfg, instanceID, scripts.Deployment()); err != nil {
		return err
	}

	return nil
}

func SelectAWSRegion(ctx context.Context, cfg aws.Config) (string, error) {
	client := ec2.NewFromConfig(cfg)

	result, err := client.DescribeRegions(
		ctx,
		&ec2.DescribeRegionsInput{},
	)
	if err != nil {
		return "", fmt.Errorf("list AWS regions: %w", err)
	}

	// Map the pretty display name back to the real region code.
	regionMap := make(map[string]string)
	options := make([]string, 0, len(result.Regions))

	for _, region := range result.Regions {
		regionName := aws.ToString(region.RegionName)

		if regionName == "" {
			continue
		}

		geography := ""

		if len(region.Geography) > 0 {
			geography = aws.ToString(region.Geography[0].Name)
		}

		displayName := regionName

		if geography != "" {
			displayName = fmt.Sprintf(
				"%s (%s)",
				geography,
				regionName,
			)
		}

		options = append(options, displayName)
		regionMap[displayName] = regionName
	}

	selected, err := SelectRegion(options)
	if err != nil {
		return "", fmt.Errorf("select AWS region: %w", err)
	}

	regionName, ok := regionMap[selected]
	if !ok {
		return "", fmt.Errorf("could not determine region from selection")
	}

	return regionName, nil
}

func SelectRegion(regions []string) (string, error) {
	var selected string

	prompt := &survey.Select{
		Message: "Select AWS region:",
		Options: regions,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return "", err
	}

	return selected, nil
}

func FindLatestAMI(
	ctx context.Context,
	cfg aws.Config,
) (string, error) {

	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{
				Name: aws.String("name"),
				Values: []string{
					"al2023-ami-2023.*-kernel-*-x86_64",
				},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"x86_64"},
			},
		},
	}

	result, err := client.DescribeImages(ctx, input)
	if err != nil {
		return "", fmt.Errorf("describe images: %w", err)
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf(
			"no Amazon Linux 2023 AMIs found in %s",
			cfg.Region,
		)
	}

	latest := result.Images[0]

	for _, image := range result.Images[1:] {
		if aws.ToString(image.CreationDate) >
			aws.ToString(latest.CreationDate) {

			latest = image
		}
	}

	imageID := aws.ToString(latest.ImageId)

	fmt.Println("Selected AMI:")
	fmt.Println("AMI ID:", imageID)
	fmt.Println("Name:", aws.ToString(latest.Name))
	fmt.Println(
		"Created:",
		aws.ToString(latest.CreationDate),
	)

	return imageID, nil
}

func CreateInstance(
	ctx context.Context,
	cfg aws.Config,
	imageID string,
	securityGroupID string,
) (string, error) {

	client := ec2.NewFromConfig(cfg)
	localIP, err := getOutboundIP()
	if err != nil {
		return "", fmt.Errorf("get outbound IP: %w", err)
	}

	log.Printf("Outbound IP: %s\n", localIP.String())

	input := &ec2.RunInstancesInput{
		ImageId:          aws.String(imageID),
		InstanceType:     ec2types.InstanceTypeT3Micro,
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		SecurityGroupIds: []string{securityGroupID},

		IamInstanceProfile: &ec2types.IamInstanceProfileSpecification{
			Name: aws.String("wiregam-ec2-ssm-role"),
		},

		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String("wiregam"),
					},
					{
						Key:   aws.String("Project"),
						Value: aws.String("wiregam-instance"),
					},
				},
			},
		},
	}

	result, err := client.RunInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf(
			"failed to create EC2 instance: %w",
			err,
		)
	}

	if len(result.Instances) == 0 {
		return "", fmt.Errorf(
			"AWS returned no EC2 instances",
		)
	}

	instanceID := aws.ToString(
		result.Instances[0].InstanceId,
	)

	fmt.Printf(
		"Successfully launched EC2 instance: %s\n",
		instanceID,
	)

	return instanceID, nil
}

func WaitForInstance(
	ctx context.Context,
	cfg aws.Config,
	instanceID string,
) error {

	client := ec2.NewFromConfig(cfg)

	fmt.Println("Waiting for EC2 instance to start...")

	waiter := ec2.NewInstanceRunningWaiter(client)

	err := waiter.Wait(
		ctx,
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		},
		5*time.Minute,
	)

	if err != nil {
		return fmt.Errorf(
			"waiting for EC2 instance %s: %w",
			instanceID,
			err,
		)
	}

	fmt.Println("EC2 instance is running.")

	return nil
}

func WaitForSSM(
	ctx context.Context,
	cfg aws.Config,
	instanceID string,
) error {

	client := ssm.NewFromConfig(cfg)

	fmt.Println(
		"Waiting for instance to register with SSM...",
	)

	for attempt := 1; attempt <= 60; attempt++ {

		result, err := client.DescribeInstanceInformation(
			ctx,
			&ssm.DescribeInstanceInformationInput{
				Filters: []ssmtypes.InstanceInformationStringFilter{
					{
						Key: aws.String("InstanceIds"),
						Values: []string{
							instanceID,
						},
					},
				},
			},
		)

		if err != nil {
			return fmt.Errorf(
				"checking SSM registration: %w",
				err,
			)
		}

		if len(result.InstanceInformationList) > 0 {
			info := result.InstanceInformationList[0]

			fmt.Printf(
				"SSM instance status: %s\n",
				info.PingStatus,
			)

			if info.PingStatus == ssmtypes.PingStatusOnline {
				fmt.Println("Instance is ready for SSM.")
				return nil
			}
		}

		fmt.Printf(
			"Waiting for SSM... attempt %d/60\n",
			attempt,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf(
		"instance %s did not become available in SSM",
		instanceID,
	)
}

func RunDeploymentScripts(
	ctx context.Context,
	cfg aws.Config,
	instanceID string,
	deploymentScripts []scripts.Script,
) error {
	client := ssm.NewFromConfig(cfg)

	for _, script := range deploymentScripts {
		if strings.TrimSpace(script.Name) == "" {
			return errors.New("deployment script has an empty name")
		}

		if strings.TrimSpace(script.Contents) == "" {
			return fmt.Errorf("deployment script %q is empty", script.Name)
		}

		if err := runSSMScript(ctx, client, instanceID, script); err != nil {
			return err
		}
	}

	return nil
}

func runSSMScript(
	ctx context.Context,
	client *ssm.Client,
	instanceID string,
	script scripts.Script,
) error {
	fmt.Printf("Running %s on %s through SSM...\n", script.Name, instanceID)

	output, err := client.SendCommand(
		ctx,
		&ssm.SendCommandInput{
			DocumentName: aws.String("AWS-RunShellScript"),
			InstanceIds: []string{
				instanceID,
			},
			Parameters: map[string][]string{
				"commands": {
					encodeShellScript(script.Contents),
				},
			},
			Comment: aws.String("WireGam: " + script.Name),
		},
	)

	if err != nil {
		return fmt.Errorf("send %s through SSM: %w", script.Name, err)
	}

	if output.Command == nil {
		return fmt.Errorf("SSM returned no command information for %s", script.Name)
	}

	commandID := aws.ToString(output.Command.CommandId)
	if commandID == "" {
		return fmt.Errorf("SSM returned an empty command ID for %s", script.Name)
	}

	invocation, err := waitForCommand(ctx, client, commandID, instanceID)
	if err != nil {
		return fmt.Errorf("wait for %s: %w", script.Name, err)
	}

	stdout := strings.TrimSpace(aws.ToString(invocation.StandardOutputContent))
	stderr := strings.TrimSpace(aws.ToString(invocation.StandardErrorContent))
	status := invocation.Status

	if status != ssmtypes.CommandInvocationStatusSuccess {
		return fmt.Errorf(
			"script %s failed with SSM status %s\nstdout:\n%s\nstderr:\n%s",
			script.Name,
			status,
			stdout,
			stderr,
		)
	}

	fmt.Printf("Completed %s.\n", script.Name)
	if stdout != "" {
		fmt.Printf("%s\n", stdout)
	}

	return nil
}

func encodeShellScript(contents string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(contents))
	return "printf '%s' '" + encoded + "' | base64 --decode | /usr/bin/env bash"
}

func waitForCommand(
	ctx context.Context,
	client *ssm.Client,
	commandID string,
	instanceID string,
) (*ssm.GetCommandInvocationOutput, error) {
	const maxAttempts = 60

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := client.GetCommandInvocation(
			ctx,
			&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(instanceID),
			},
		)
		if err == nil {
			switch output.Status {
			case ssmtypes.CommandInvocationStatusPending,
				ssmtypes.CommandInvocationStatusInProgress,
				ssmtypes.CommandInvocationStatusDelayed:
				// Keep polling until the command reaches a terminal status.
			default:
				return output, nil
			}
		} else {
			var apiErr smithy.APIError
			if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvocationDoesNotExist" {
				return nil, err
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil, fmt.Errorf("command %s did not finish within two minutes", commandID)
}

func CreateSecurityGroup(
	ctx context.Context,
	cfg aws.Config,
	groupName string,
	description string,
) (string, error) {

	client := ec2.NewFromConfig(cfg)
	existing, err := client.DescribeSecurityGroups(
		ctx,
		&ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String("group-name"),
					Values: []string{groupName},
				},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("describe security groups: %w", err)
	}

	if len(existing.SecurityGroups) > 0 {
		securityGroupID := aws.ToString(existing.SecurityGroups[0].GroupId)
		fmt.Println("Using existing security group:", securityGroupID)
		return securityGroupID, nil
	}

	result, err := client.CreateSecurityGroup(
		ctx,
		&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(groupName),
			Description: aws.String(description),
		},
	)
	if err != nil {
		return "", fmt.Errorf("create security group: %w", err)
	}

	securityGroupID := aws.ToString(result.GroupId)

	fmt.Println("Created security group:", securityGroupID)
	return securityGroupID, nil
}

func customizeSecurityGroupPorts(ctx context.Context, client *ec2.Client, securityGroupID string, localIP net.IP) error {

	// localIP, err := getOutboundIP()
	// if err != nil {
	// 	return fmt.Errorf("get outbound IP: %w", err)
	// }

	// log.Printf("Outbound IP: %s\n", localIP.String())
	_, err := client.AuthorizeSecurityGroupIngress(
		ctx,
		&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(securityGroupID),
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("udp"),
					FromPort:   aws.Int32(51820),
					ToPort:     aws.Int32(51820),
					IpRanges: []ec2types.IpRange{
						{
							CidrIp: aws.String(fmt.Sprintf("%s/32", localIP.String())),
						},
					},
				},
			},
		},
	)

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) &&
		apiErr.ErrorCode() == "InvalidPermission.Duplicate" {
		fmt.Println("UDP port 51820 rule already exists.")
		return nil
	}

	return fmt.Errorf("allow UDP port 51820: %w", err)
	// if err != nil {
	// 	return fmt.Errorf("allow UDP port 51820: %w", err)
	// }
	return nil
}

func getOutboundIP() (net.IP, error) {
	// 8.8.8.8:80 is a dummy address (Google DNS). No connection is actually made.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}
