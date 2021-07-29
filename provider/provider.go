package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/renproject/multichain"
	"github.com/renproject/nodectl/renvm"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
)

const MaxNameLength = 32

var DarknodeService = `[Unit]
Description=RenVM Darknode Daemon

[Service]
WorkingDirectory=/home
ExecStart=/home/darknode/.darknode/bin/darknode --config /home/darknode/.darknode/config.json
Restart=on-failure
PrivateTmp=true
NoNewPrivileges=true

# Specifies which signal to use when killing a service. Defaults to SIGTERM.
# SIGHUP gives parity time to exit cleanly before SIGKILL (default 90s)
KillSignal=SIGHUP

[Install]
WantedBy=default.target
`

var (
	ErrEmptyName = errors.New("node name cannot be empty")

	ErrNameTooLong = errors.New("name exceeds max name length")

	ErrMissingCredential = errors.New("missing cloud provider credential for deployment")

	ErrUnknownProvider = errors.New("unknown cloud provider")

	ErrInstanceTypeNotAvailable = errors.New("selected instance type is not available")

	ErrRegionNotAvailable = errors.New("selected region is not available")

	ErrNoAvailableRegion = errors.New("cannot find any available region with given account")

	ErrInsufficientPermission = errors.New("insufficient permissions")

	ErrInvalidNodeNameForGCP = errors.New("for google cloud, name must start with a lowercase letter followed by up to 62 lowercase letters, numbers, or hyphens, and cannot end with a hyphen")
)

var (
	NameAws = "aws"
	NameDo  = "do"
	NameGcp = "gcp"
)

type Provider interface {

	// Name of the Provider
	Name() string

	// Deploy darknode with from this provider
	Deploy(ctx *cli.Context) error
}

func ParseProvider(ctx *cli.Context) (Provider, error) {
	if ctx.Bool(NameAws) {
		return NewAWS(ctx)
	}

	if ctx.Bool(NameDo) {
		return NewDo(ctx)
	}

	// todo
	// if ctx.Bool(NameGcp) {
	// 	return NewGCP(ctx)
	// }

	return nil, ErrUnknownProvider
}

// Validate the params which are general to all providers.
func validateCommonParams(ctx *cli.Context) error {
	// Check the name is not empty and not exceeding max length
	name := ctx.String("name")
	if name == "" {
		return ErrEmptyName
	}
	if len(name) > MaxNameLength {
		return ErrNameTooLong
	}

	// Check the name isn't used.
	if _, err := os.Stat(util.NodePath(name)); err == nil {
		return fmt.Errorf("node [%v] already exist", name)
	}

	// Check the network
	network := multichain.Network(ctx.String("network"))
	switch network {
	case multichain.NetworkMainnet:
	case multichain.NetworkTestnet:
	case multichain.NetworkDevnet:
	case multichain.NetworkLocalnet:
	default:
		return errors.New("unknown RenVM network")
	}

	// Verify the config file if user wants to use their own config
	configFile := ctx.String("config")
	if configFile != "" {
		// verify the config exist and of the right format
		path, err := filepath.Abs(configFile)
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err != nil {
			return errors.New("config file doesn't exist")
		}
		_, err = renvm.NewOptionsFromFile(path)
		if err != nil {
			return fmt.Errorf("incompatible config, err = %v", err)
		}
	}
	return nil
}

// initialize files for deploying a Darknode
func initialize(ctx *cli.Context, opts renvm.Options) error {
	name := ctx.String("name")
	network := multichain.Network(ctx.String("network"))
	path := util.NodePath(name)

	// Create directory for the Darknode
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	// Create `tags.out` file
	tags := []byte(strings.TrimSpace(ctx.String("tags")))
	tagsPath := filepath.Join(path, "tags.out")
	if err := ioutil.WriteFile(tagsPath, tags, 0600); err != nil {
		return err
	}

	// Create `ssh_keypair` and `ssh_keypair.pub` files for the remote instance
	if err := util.GenerateSshKeyAndWriteToDir(name); err != nil {
		return err
	}

	// Generate the `darknode.service` file
	servicePath := filepath.Join(path, "darknode.service")
	if err := ioutil.WriteFile(servicePath, []byte(DarknodeService), 0600); err != nil {
		return err
	}

	// Create the `genesis.json` file
	state, err := renvm.GenesisFile(network)
	if err != nil {
		return err
	}
	stateBytes, err := json.MarshalIndent(state, "", "    ")
	if err != nil {
		return err
	}
	statePath := filepath.Join(path, "genesis.json")
	err = ioutil.WriteFile(statePath, stateBytes, 0600)
	return err
}

func applyTerraform(name string) error {
	init := fmt.Sprintf("cd %v && terraform init", util.NodePath(name))
	if err := util.Run("bash", "-c", init); err != nil {
		return err
	}
	apply := fmt.Sprintf("cd %v && terraform apply -auto-approve -no-color", util.NodePath(name))
	return util.Run("bash", "-c", apply)
}
