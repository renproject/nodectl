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
	// todo
	// if ctx.Bool(NameAws) {
	// 	return NewAWS(ctx)
	// }

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

		jsonFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer jsonFile.Close()
		var config renvm.Options
		if err := json.NewDecoder(jsonFile).Decode(&config); err != nil {
			return fmt.Errorf("incompatible config, err = %v", err)
		}
	}
	return nil
}

// initialize files for deploying a darknode
func initialize(ctx *cli.Context) error {
	name := ctx.String("name")
	path := util.NodePath(name)

	// Create directory for the Ren node
	if err := os.Mkdir(path, 0700); err != nil {
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

	// Create `config.json` file
	configFile := ctx.String("config")
	destination := filepath.Join(util.NodePath(name), "config.json")
	if configFile != "" {
		path, err := filepath.Abs(configFile)
		if err != nil {
			return err
		}
		input, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(destination, input, 0600)
	} else {
		network := multichain.Network(ctx.String("network"))
		options := renvm.NewOptions(network)
		configData, err := json.MarshalIndent(options, "", "    ")
		if err != nil {
			return err
		}
		return ioutil.WriteFile(destination, configData, 0600)
	}

	// TODO : Create `genesis.json` file
}
