package nodectl

import (
	"github.com/renproject/nodectl/provider"
	"github.com/urfave/cli/v2"
)

// General flags
var (
	NameFlag = &cli.StringFlag{
		Name:  "name",
		Usage: "A unique human-readable `string` for identifying the darknode",
	}
	TagsFlag = &cli.StringFlag{
		Name:  "tags",
		Usage: "Multiple human-readable comma separated `strings` for identifying groups of darknodeS",
	}
	ConfigFlag = &cli.StringFlag{
		Name:  "config",
		Usage: "Config file for your darknode",
	}
	SnapshotFlag = &cli.StringFlag{
		Name:  "snapshot",
		Usage: "Snapshot of the darknode to recover",
	}
	NetworkFlag = &cli.StringFlag{
		Name:        "network",
		Value:       "mainnet",
		Usage:       "Network of RenVM you want to join",
		DefaultText: "mainnet",
	}
	VersionFlag = &cli.StringFlag{
		Name:  "version",
		Usage: "Version of darknode you want to upgrade to",
	}
	DowngradeFlag = &cli.BoolFlag{
		Name:  "downgrade",
		Usage: "Force downgrading to an older version without interactive prompts",
	}
	ForceFlag = &cli.BoolFlag{
		Name:    "force",
		Aliases: []string{"f"},
		Usage:   "Force destruction without interactive prompts",
	}
	VerboseFlag = &cli.BoolFlag{
		Name:    "verbose",
		Aliases: []string{"v"},
		Usage:   "Show additional details of darknodes",
	}
)

// AWS flags
var (
	AwsFlag = &cli.BoolFlag{
		Name:  provider.NameAws,
		Usage: "AWS will be used to provision the darknode",
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
		Name:        "aws-region",
		Usage:       "An optional AWS region",
		DefaultText: "random",
	}
	AwsInstanceFlag = &cli.StringFlag{
		Name:        "aws-instance",
		Value:       provider.DefaultAWSInstance,
		Usage:       "An optional AWS EC2 instance type",
		DefaultText: provider.DefaultAWSInstance,
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
		Usage: "Digital Ocean will be used to provision the darknode",
	}
	DoTokenFlag = &cli.StringFlag{
		Name:  "do-token",
		Usage: "Digital Ocean API token for programmatic access",
	}
	DoRegionFlag = &cli.StringFlag{
		Name:        "do-region",
		Usage:       "An optional Digital Ocean region",
		DefaultText: "random",
	}
	DoSizeFlag = &cli.StringFlag{
		Name:        "do-droplet",
		Value:       provider.DefaultDigitalOceanDroplet,
		Usage:       "An optional Digital Ocean droplet size",
		DefaultText: "Basic 1CPU/1G/25G",
	}
)

// Google cloud platform flags
var (
	GcpFlag = &cli.BoolFlag{
		Name:  provider.NameGcp,
		Usage: "Google Cloud Platform will be used to provision the darknode",
	}
	GcpCredFlag = &cli.StringFlag{
		Name:  "gcp-credentials",
		Usage: "Path of the Service Account credential file (JSON) to be used",
	}
	GcpMachineFlag = &cli.StringFlag{
		Name:        "gcp-machine",
		Value:       "n1-standard-1",
		Usage:       "An optional Google Cloud machine type",
		DefaultText: "n1-standard-1",
	}
	GcpRegionFlag = &cli.StringFlag{
		Name:        "gcp-region",
		Usage:       "An optional Google Cloud Region",
		DefaultText: "random",
	}
)
