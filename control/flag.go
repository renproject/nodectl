package darknode

import (
	"github.com/renproject/darknode-cli/darknode/provider"
	"github.com/urfave/cli/v2"
)

// General flags
var (
	NameFlag = &cli.StringFlag{
		Name:  "name",
		Usage: "A unique human-readable `string` for identifying the Darknode",
	}
	TagsFlag = &cli.StringFlag{
		Name:  "tags",
		Usage: "Multiple human-readable comma separated `strings` for identifying groups of Darknodes",
	}
	ConfigFlag = &cli.StringFlag{
		Name:  "config",
		Usage: "Config file for your Ren node",
	}
	ScriptFlag = &cli.StringFlag{
		Name:  "script",
		Usage: "A `string` containing commands you want the Darknode to run",
	}
	NetworkFlag = &cli.StringFlag{
		Name:  "network",
		Value: "mainnet",
		Usage: "Network of your Darknode (default: mainnet)",
	}
	AddressFlag = &cli.StringFlag{
		Name:  "address",
		Usage: "Ethereum address you want to withdraw the tokens to",
	}
	FileFlag = &cli.StringFlag{
		Name:  "file",
		Usage: "Path of the script file you want the Darknode to run",
	}
	VersionFlag = &cli.StringFlag{
		Name:  "version",
		Usage: "Version of darknode you want to upgrade to",
	}
	InstanceFlag = &cli.StringFlag{
		Name:  "instance",
		Usage: "Instance type you want to resize to",
	}
	DowngradeFlag = &cli.BoolFlag{
		Name:  "downgrade",
		Usage: "Force downgrading to an older version without interactive prompts",
	}
	ForceFlag = &cli.BoolFlag{
		Name:  "force, f",
		Usage: "Force destruction without interactive prompts",
	}
	StorageFlag = &cli.IntFlag{
		Name:  "storage",
		Usage: "Size of the storage you want to upgrade to",
	}
)

// AWS flags
var (
	AwsFlag = &cli.BoolFlag{
		Name:  provider.NameAws,
		Usage: "AWS will be used to provision the Darknode",
	}
	AwsAccessKeyFlag = &cli.StringFlag{
		Name:  "aws-access-key",
		Usage: "AWS access `key` for programmatic access",
	}
	AwsSecretKeyFlag = &cli.StringFlag{
		Name:  "aws-secret-key",
		Usage: "AWS secret `key` for programmatic access",
	}
	AwsRegionFlag = &cli.StringFlag{
		Name:  "aws-region",
		Usage: "An optional AWS region (default: random)",
	}
	AwsInstanceFlag = &cli.StringFlag{
		Name:  "aws-instance",
		Value: "t3.micro",
		Usage: "An optional AWS EC2 instance type (default: t3.micro)",
	}
	AwsProfileFlag = &cli.StringFlag{
		Name:  "aws-profile",
		Value: "default",
		Usage: "Name of the profile containing the credentials",
	}
)

// Digital ocean flags
var (
	DoFlag = &cli.BoolFlag{
		Name:  provider.NameDo,
		Usage: "Digital Ocean will be used to provision the Darknode",
	}
	DoTokenFlag = &cli.StringFlag{
		Name:  "do-token",
		Usage: "Digital Ocean API token for programmatic access",
	}
	DoRegionFlag = &cli.StringFlag{
		Name:  "do-region",
		Usage: "An optional Digital Ocean region (default: random)",
	}
	DoSizeFlag = &cli.StringFlag{
		Name:  "do-droplet",
		Value: "s-1vcpu-1gb",
		Usage: "An optional Digital Ocean droplet size (default: s-1vcpu-1gb)",
	}
)

// Google cloud platform flags
var (
	GcpFlag = &cli.BoolFlag{
		Name:  provider.NameGcp,
		Usage: "Google Cloud Platform will be used to provision the Darknode",
	}
	GcpCredFlag = &cli.StringFlag{
		Name:  "gcp-credentials",
		Usage: "Path of the Service Account credential file (JSON) to be used",
	}
	GcpMachineFlag = &cli.StringFlag{
		Name:  "gcp-machine",
		Value: "n1-standard-1",
		Usage: "An optional Google Cloud machine type (default: n1-standard-1)",
	}
	GcpRegionFlag = &cli.StringFlag{
		Name:  "gcp-region",
		Usage: "An optional Google Cloud Region (default: random)",
	}
)
