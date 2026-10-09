# WireGam

> **Beta software — use at your own risk.** WireGam is still under active development. It creates and configures AWS resources in your account. Those resources can incur AWS charges, and this project does not yet clean them up automatically. Review all resources and billing in the AWS Console after every test deployment.

WireGam is a command-line tool that provisions an AWS EC2 instance and connects a Fedora laptop to it through WireGuard in one deployment command.

## What the AWS deployment currently does

For the AWS region you select, WireGam:

- installs the local WireGuard and DNS helper packages when needed;
- creates or reuses a laptop key pair under `~/.wiregam/wireguard/`;
- finds the latest Amazon Linux 2023 x86_64 AMI;
- finds or creates the `wiregam-sg` security group;
- permits UDP port `51820` from the IP address detected on the machine running WireGam;
- launches a `t3.micro` EC2 instance tagged `Name=wiregam` and `Project=wiregam-instance`;
- attaches the existing instance profile named `wiregam-ec2-ssm-role`;
- disables source/destination checking so the instance can forward VPN traffic;
- waits for the instance and its SSM agent;
- runs embedded scripts that install, configure, start, and verify WireGuard on EC2;
- writes `/etc/wireguard/wg0.conf` on the laptop; and
- starts the laptop tunnel, verifies its first handshake, and enables it at boot.

The laptop private key never leaves the laptop. Only its public key is sent to
EC2. The EC2 private key likewise remains on EC2; the deploy command retrieves
only its public key.

The deployment scripts are embedded in the CLI under `internal/scripts/`. They
run in the order listed by `scripts.Deployment()`; WireGam waits for each script
to succeed before starting the next one.

The current implementation does not choose a VPC or subnet. AWS therefore uses the default VPC/networking for the selected region. A region or account without a default VPC may fail to deploy.

## Costs and safety

Launching an EC2 instance, storage, public IPv4 addresses, network egress, and other AWS services can incur charges. Free Tier eligibility is not guaranteed and depends on your AWS account and region.

Before running WireGam:

- use a non-production AWS account where possible;
- set an AWS Budget and billing alerts;
- confirm that the selected region is intended; and
- understand that you are responsible for stopping and terminating resources WireGam creates.

Do not commit, share, or paste the AWS credentials requested by the CLI.

## Prerequisites

- Go **1.26.5** or later;
- a Fedora Linux laptop with `sudo` access and the `dnf` package manager;
- an AWS account;
- an IAM access key for an IAM user or role with the permissions below;
- an existing EC2 instance profile named `wiregam-ec2-ssm-role`; and
- internet access or VPC endpoints that allow the launched instance to reach AWS Systems Manager.

The `wiregam-ec2-ssm-role` instance profile must have the AWS-managed policy `AmazonSSMManagedInstanceCore` attached. It is the role used by the EC2 instance, not the credentials you enter into WireGam.

## Run WireGam

Clone the repository, then build and run the CLI:

```bash
go build -o wiregam .
./wiregam auth --provider aws
./wiregam deploy --provider aws
```

`deploy` may ask for the laptop's sudo password while installing local packages
and writing `/etc/wireguard/wg0.conf`. When it returns successfully, `wg0` is up
and all IPv4 traffic is routed through the new EC2 instance.

`auth` prompts for an AWS access key and secret access key, validates them with STS, and stores them locally at:

```text
~/.wiregam/creds/aws/aws.json
```

The credentials directory is created with owner-only permissions. Remove that file when you are done testing or rotate the access key in AWS.

You can also run the commands without building a binary:

```bash
go run . auth --provider aws
go run . deploy --provider aws
```

## IAM permissions for the WireGam credentials

Replace `ACCOUNT_ID` with your AWS account ID before attaching this policy to the IAM identity whose access keys you provide to WireGam. This is a functional starter policy for the AWS operations WireGam currently performs; tighten it further for your account before production use.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowWireGamEC2DiscoveryAndDeployment",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeRegions",
        "ec2:DescribeImages",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeInstances",
        "ec2:CreateSecurityGroup",
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:ModifyInstanceAttribute",
        "ec2:RunInstances",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowWireGamSSMDocument",
      "Effect": "Allow",
      "Action": "ssm:SendCommand",
      "Resource": "arn:aws:ssm:*::document/AWS-RunShellScript"
    },
    {
      "Sid": "AllowCommandsOnWireGamInstances",
      "Effect": "Allow",
      "Action": "ssm:SendCommand",
      "Resource": "arn:aws:ec2:*:ACCOUNT_ID:instance/*",
      "Condition": {
        "StringEquals": {
          "ssm:resourceTag/Project": "wiregam-instance"
        }
      }
    },
    {
      "Sid": "AllowSSMStatusChecks",
      "Effect": "Allow",
      "Action": [
        "ssm:DescribeInstanceInformation",
        "ssm:GetCommandInvocation"
      ],
      "Resource": "*"
    },
    {
      "Sid": "AllowWireGamToPassEC2Role",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": "arn:aws:iam::ACCOUNT_ID:role/wiregam-ec2-ssm-role",
      "Condition": {
        "StringEquals": {
          "iam:PassedToService": "ec2.amazonaws.com"
        }
      }
    }
  ]
}
```

`ec2:Describe*` actions must use `"Resource": "*"` in an IAM policy. The starter policy also deliberately uses `"Resource": "*"` for the EC2 deployment actions because `RunInstances` has several dependent AWS resources. Scope the policy with region, AMI, VPC, subnet, instance-type, and request-tag conditions once the networking and deployment inputs are explicit.

## Troubleshooting

### `InvalidGroup.Duplicate`

The `wiregam-sg` security group already exists. WireGam looks up an existing group by name before trying to create one. If you are running an older build, rebuild it before retrying.

### `InvalidPermission.Duplicate`

UDP port `51820` is already permitted for the detected source IP. This means the requested security-group rule is already present.

### Instance never becomes available in SSM

Check all of the following:

- the `wiregam-ec2-ssm-role` instance profile exists and has `AmazonSSMManagedInstanceCore`;
- the EC2 instance has outbound access to SSM, EC2 Messages, and SSM Messages endpoints, either through the internet/NAT or VPC interface endpoints; and
- the selected region has default networking available.

## Cleanup

When finished, manually terminate the EC2 instances tagged `Project=wiregam-instance`. Review and delete the `wiregam-sg` security group only after no instances or network interfaces use it. Also review EBS volumes, elastic IPs, snapshots, and other related resources in the AWS Console.

## Beta status

WireGam is beta software. Interfaces, resource names, IAM requirements, and deployment behavior may change without notice. Do not rely on it for production VPN access without an independent security review and operational testing.

## License

Copyright © 2026 Caleb Ajibade.

WireGam is licensed under the [GNU General Public License v3.0](LICENSE) (GPL-3.0-only). You may use, modify, and distribute it under the terms of that license. Distributed derivative works must also be made available under GPLv3.
