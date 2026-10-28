package aws

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type fakeDestroyEC2Client struct {
	describeInputs  []*ec2.DescribeInstancesInput
	describeOutputs []*ec2.DescribeInstancesOutput
	describeErr     error
	terminateInputs []*ec2.TerminateInstancesInput
	terminateErr    error
}

func (f *fakeDestroyEC2Client) DescribeInstances(
	_ context.Context,
	input *ec2.DescribeInstancesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	f.describeInputs = append(f.describeInputs, input)
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	output := f.describeOutputs[0]
	f.describeOutputs = f.describeOutputs[1:]
	return output, nil
}

func (f *fakeDestroyEC2Client) TerminateInstances(
	_ context.Context,
	input *ec2.TerminateInstancesInput,
	_ ...func(*ec2.Options),
) (*ec2.TerminateInstancesOutput, error) {
	f.terminateInputs = append(f.terminateInputs, input)
	if f.terminateErr != nil {
		return nil, f.terminateErr
	}
	return &ec2.TerminateInstancesOutput{}, nil
}

func TestFindWireGamInstancesUsesExactTagAndPagination(t *testing.T) {
	client := &fakeDestroyEC2Client{
		describeOutputs: []*ec2.DescribeInstancesOutput{
			{
				Reservations: []types.Reservation{{
					Instances: []types.Instance{{InstanceId: aws.String("i-first")}},
				}},
				NextToken: aws.String("next-page"),
			},
			{
				Reservations: []types.Reservation{{
					Instances: []types.Instance{{InstanceId: aws.String("i-second")}},
				}},
			},
		},
	}

	got, err := findWireGamInstances(context.Background(), client)
	if err != nil {
		t.Fatalf("findWireGamInstances() error = %v", err)
	}
	if want := []string{"i-first", "i-second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("findWireGamInstances() = %v, want %v", got, want)
	}
	if len(client.describeInputs) != 2 {
		t.Fatalf("DescribeInstances calls = %d, want 2", len(client.describeInputs))
	}

	firstInput := client.describeInputs[0]
	assertFilter(t, firstInput.Filters, "tag:Project", []string{"wiregam-instance"})
	assertFilter(t, firstInput.Filters, "instance-state-name", []string{"pending", "running", "stopping", "stopped"})
	if got := aws.ToString(client.describeInputs[1].NextToken); got != "next-page" {
		t.Fatalf("second page token = %q, want %q", got, "next-page")
	}
}

func TestDestroyInstancesRequiresConfirmation(t *testing.T) {
	client := &fakeDestroyEC2Client{
		describeOutputs: []*ec2.DescribeInstancesOutput{{
			Reservations: []types.Reservation{{
				Instances: []types.Instance{{InstanceId: aws.String("i-wiregam")}},
			}},
		}},
	}
	confirmationCalled := false

	err := destroyInstances(context.Background(), client, "us-east-1", func(ids []string, region string) (bool, error) {
		confirmationCalled = true
		if !reflect.DeepEqual(ids, []string{"i-wiregam"}) {
			t.Fatalf("confirmation IDs = %v", ids)
		}
		if region != "us-east-1" {
			t.Fatalf("confirmation region = %q", region)
		}
		return false, nil
	}, disconnectShouldNotRun(t))
	if err != nil {
		t.Fatalf("destroyInstances() error = %v", err)
	}
	if !confirmationCalled {
		t.Fatal("confirmation was not requested")
	}
	if len(client.terminateInputs) != 0 {
		t.Fatalf("TerminateInstances calls = %d, want 0", len(client.terminateInputs))
	}
}

func TestDestroyInstancesTerminatesOnlyPreviewedInstances(t *testing.T) {
	client := &fakeDestroyEC2Client{
		describeOutputs: []*ec2.DescribeInstancesOutput{{
			Reservations: []types.Reservation{{
				Instances: []types.Instance{
					{InstanceId: aws.String("i-one")},
					{InstanceId: aws.String("i-two")},
				},
			}},
		}},
	}

	disconnected := false
	err := destroyInstances(context.Background(), client, "eu-west-1", func([]string, string) (bool, error) {
		return true, nil
	}, func(context.Context) (bool, error) {
		disconnected = true
		return true, nil
	})
	if err != nil {
		t.Fatalf("destroyInstances() error = %v", err)
	}
	if len(client.terminateInputs) != 1 {
		t.Fatalf("TerminateInstances calls = %d, want 1", len(client.terminateInputs))
	}
	if !disconnected {
		t.Fatal("active VPN was not disconnected before termination")
	}
	if got, want := client.terminateInputs[0].InstanceIds, []string{"i-one", "i-two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("terminated IDs = %v, want %v", got, want)
	}
}

func TestDestroyInstancesReturnsDescribeError(t *testing.T) {
	client := &fakeDestroyEC2Client{describeErr: errors.New("AWS unavailable")}
	err := destroyInstances(context.Background(), client, "us-east-1", func([]string, string) (bool, error) {
		t.Fatal("confirmation should not be requested after a lookup error")
		return false, nil
	}, disconnectShouldNotRun(t))
	if err == nil {
		t.Fatal("destroyInstances() error = nil, want an error")
	}
}

func TestDestroyInstancesDoesNotTerminateWhenDisconnectFails(t *testing.T) {
	client := &fakeDestroyEC2Client{
		describeOutputs: []*ec2.DescribeInstancesOutput{{
			Reservations: []types.Reservation{{
				Instances: []types.Instance{{InstanceId: aws.String("i-wiregam")}},
			}},
		}},
	}

	err := destroyInstances(context.Background(), client, "us-east-1", func([]string, string) (bool, error) {
		return true, nil
	}, func(context.Context) (bool, error) {
		return true, errors.New("could not stop wg0")
	})
	if err == nil {
		t.Fatal("destroyInstances() error = nil, want an error")
	}
	if len(client.terminateInputs) != 0 {
		t.Fatalf("TerminateInstances calls = %d, want 0", len(client.terminateInputs))
	}
}

func disconnectShouldNotRun(t *testing.T) disconnectLocalVPN {
	t.Helper()
	return func(context.Context) (bool, error) {
		t.Fatal("disconnect should not be called")
		return false, nil
	}
}

func assertFilter(t *testing.T, filters []types.Filter, name string, values []string) {
	t.Helper()
	for _, filter := range filters {
		if aws.ToString(filter.Name) == name {
			if !reflect.DeepEqual(filter.Values, values) {
				t.Fatalf("filter %q values = %v, want %v", name, filter.Values, values)
			}
			return
		}
	}
	t.Fatalf("filter %q was not provided", name)
}
