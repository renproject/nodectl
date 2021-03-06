package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/renproject/aw/wire"
	"github.com/renproject/multichain"
	"github.com/renproject/nodectl/renvm"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
	"github.com/zclconf/go-cty/cty"
)

const DefaultDigitalOceanDroplet = "s-1vcpu-1gb"

type providerDO struct {
	token string
}

// NewDo creates a Digital Ocean provider.
func NewDo(ctx *cli.Context) (Provider, error) {
	token := ctx.String("do-token")
	if token == "" {
		return nil, ErrMissingCredential
	}

	return providerDO{
		token: token,
	}, nil
}

// Name implements the `Provider` interface
func (p providerDO) Name() string {
	return NameDo
}

// Deploy implements the `Provider` interface
func (p providerDO) Deploy(ctx *cli.Context) error {
	// Validate all input params
	if err := validateCommonParams(ctx); err != nil {
		return err
	}
	name := ctx.String("name")
	network := multichain.Network(ctx.String("network"))
	region, droplet, err := p.validateRegionAndDroplet(ctx)
	if err != nil {
		return err
	}

	// Get the latest darknode version
	version, err := util.LatestRelease(network)
	if err != nil {
		return err
	}

	// Fetch the remote config template
	templateOpts, err := renvm.OptionTemplate(util.OptionsURL(network))
	if err != nil {
		return err
	}

	// Initialize folder and files for the node
	if err := initialize(ctx); err != nil {
		return err
	}

	// Get file version ID
	configVersionID, err := fileVersionID(fmt.Sprintf("%v/config.json", network))
	if err != nil {
		return err
	}
	snapshotVersionID, err := fileVersionID(fmt.Sprintf("%v/latest.tar.gz", network))
	if err != nil {
		return err
	}

	// Getting everything needed by terraform
	tf := doTerraform{
		Network:            network,
		Name:               name,
		Token:              p.token,
		Region:             region.Slug,
		Size:               droplet,
		PubKeyPath:         filepath.Join(util.NodePath(name), "ssh_keypair.pub"),
		PriKeyPath:         filepath.Join(util.NodePath(name), "ssh_keypair"),
		ServiceFile:        filepath.Join(util.NodePath(name), "darknode.service"),
		UpdaterServiceFile: filepath.Join(util.NodePath(name), "darknode-updater.service"),
		Version:            version,
		ConfigVersionID:    configVersionID,
		SnapshotVersionID:  snapshotVersionID,
	}

	// Deploy all the cloud services we need
	color.Green("Deploying darknode...")
	tfData := tf.GenerateTerraformConfig()
	tfFile, err := os.Create(filepath.Join(util.NodePath(name), "main.tf"))
	if err != nil {
		return err
	}
	if _, err := tfFile.Write(tfData); err != nil {
		return err
	}
	if err := applyTerraform(name); err != nil {
		return err
	}

	// Generate the config file using the ip address and template
	ip, err := util.NodeIP(name)
	if err != nil {
		return err
	}
	opts := renvm.NewOptions(network)
	ip = fmt.Sprintf("%v:18514", ip)
	addr := wire.NewUnsignedAddress(wire.TCP, ip, uint64(time.Now().UnixNano()))
	if err := addr.Sign(opts.PrivKey); err != nil {
		return fmt.Errorf("cannot sign address: %v", err)
	}
	opts.Peers = append([]wire.Address{addr}, templateOpts.Peers...)
	opts.Selectors = templateOpts.Selectors
	opts.Chains = templateOpts.Chains
	opts.Whitelist = templateOpts.Whitelist
	optionsPath := filepath.Join(util.NodePath(name), "config.json")
	if err := renvm.OptionsToFile(opts, optionsPath); err != nil {
		return err
	}

	// Upload the config file to remote instance
	data, err := json.MarshalIndent(opts, "", "    ")
	if err != nil {
		return err
	}
	copyConfig := fmt.Sprintf("echo '%s' > $HOME/.darknode/config.json", string(data))
	if err := util.RemoteRun(name, copyConfig, "darknode"); err != nil {
		return err
	}

	// Start the darknode service
	startService := "systemctl --user start darknode"
	if err := util.RemoteRun(name, startService, "darknode"); err != nil {
		return err
	}

	color.Green("Your darknode is up and running")
	return nil
}

func (p providerDO) validateRegionAndDroplet(ctx *cli.Context) (godo.Region, string, error) {
	region := strings.ToLower(strings.TrimSpace(ctx.String("do-region")))
	droplet := strings.ToLower(strings.TrimSpace(ctx.String("do-droplet")))
	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch all available regions
	client := godo.NewFromToken(p.token)
	regions, response, err := client.Regions.List(c, nil)
	if err != nil {
		return godo.Region{}, "", err
	}
	if err := util.VerifyStatusCode(response.Response, http.StatusOK); err != nil {
		return godo.Region{}, "", err
	}
	if len(regions) == 0 {
		return godo.Region{}, "", ErrNoAvailableRegion
	}

	// Validate the given region and droplet type. Will use a random region
	// if not specified.
	if region == "" {
		// Randomly select a region which has the given droplet size.
		indexes := rand.Perm(len(regions))
		for _, index := range indexes {
			if util.StringInSlice(droplet, regions[index].Sizes) {
				if regions[index].Available {
					return regions[index], droplet, nil
				}
			}
		}
		return godo.Region{}, "", fmt.Errorf("selected droplet [%v] not available across all regions", droplet)
	} else {
		for _, r := range regions {
			if r.Slug == region {
				if util.StringInSlice(droplet, r.Sizes) {
					return r, droplet, nil
				}
				return godo.Region{}, "", fmt.Errorf("selected droplet [%v] not available in region %v", droplet, region)
			}
		}
		return godo.Region{}, "", fmt.Errorf("region [%v] is not avaliable", region)
	}
}

type doTerraform struct {
	Network            multichain.Network
	Name               string
	Token              string
	Region             string
	Size               string
	PubKeyPath         string
	PriKeyPath         string
	ServiceFile        string
	UpdaterServiceFile string
	Version            string
	ConfigVersionID    string
	SnapshotVersionID  string
}

func (do doTerraform) GenerateTerraformConfig() []byte {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	reqProviderBody := rootBody.AppendNewBlock("terraform", nil).Body().AppendNewBlock("required_providers", nil).Body()
	reqProviderBody.SetAttributeValue("digitalocean", cty.ObjectVal(map[string]cty.Value{
		"source":  cty.StringVal("digitalocean/digitalocean"),
		"version": cty.StringVal("~> 2.0"),
	}))

	providerBlock := rootBody.AppendNewBlock("provider", []string{"digitalocean"})
	providerBody := providerBlock.Body()
	providerBody.SetAttributeValue("token", cty.StringVal(do.Token))

	sshKeyBlock := rootBody.AppendNewBlock("resource", []string{"digitalocean_ssh_key", "darknode"})
	sshKeyBody := sshKeyBlock.Body()
	sshKeyBody.SetAttributeValue("name", cty.StringVal(do.Name))
	pubKey := hclwrite.Tokens{
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("public_key "),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenEqual,
			Bytes:        []byte("="),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(" file"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenOParen,
			Bytes:        []byte("("),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(fmt.Sprintf("\"%v\"", do.PubKeyPath)),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCParen,
			Bytes:        []byte(")"),
			SpacesBefore: 0,
		},
	}
	sshKeyBody.AppendUnstructuredTokens(pubKey)
	sshKeyBody.AppendNewline()

	dropletBlock := rootBody.AppendNewBlock("resource", []string{"digitalocean_droplet", "darknode"})
	dropletBody := dropletBlock.Body()
	dropletBody.SetAttributeTraversal("provider", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "digitalocean",
		},
	})
	dropletBody.SetAttributeValue("image", cty.StringVal("ubuntu-20-04-x64"))
	dropletBody.SetAttributeValue("name", cty.StringVal(do.Name))
	dropletBody.SetAttributeValue("region", cty.StringVal(do.Region))
	dropletBody.SetAttributeValue("size", cty.StringVal(do.Size))
	dropletBody.SetAttributeValue("monitoring", cty.True)
	dropletBody.SetAttributeValue("resize_disk", cty.False)
	sshKeys := hclwrite.Tokens{
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("ssh_keys "),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenEqual,
			Bytes:        []byte("="),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(" "),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenOBrack,
			Bytes:        []byte("["),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("digitalocean_ssh_key.darknode.id"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCBrack,
			Bytes:        []byte("]"),
			SpacesBefore: 0,
		},
	}
	dropletBody.AppendUnstructuredTokens(sshKeys)
	dropletBody.AppendNewline()

	remoteExecBlock := dropletBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExecBody := remoteExecBlock.Body()
	remoteExecBody.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("until sudo apt update; do sleep 4; done"),
		cty.StringVal("sudo adduser darknode --gecos \",,,\" --disabled-password"),
		cty.StringVal("sudo rsync --archive --chown=darknode:darknode ~/.ssh /home/darknode"),
		cty.StringVal("curl -sSL https://repos.insights.digitalocean.com/install.sh | sudo bash"),
		cty.StringVal("until sudo apt-get install -y ufw build-essential libhwloc-dev; do sleep 4; done"),
		cty.StringVal("sudo ufw allow 22/tcp"),
		cty.StringVal("sudo ufw allow 18514/tcp"),
		cty.StringVal("sudo ufw allow 18515/tcp"),
		cty.StringVal("sudo ufw --force enable"),
		cty.StringVal("wget https://github.com/CosmWasm/wasmvm/archive/v0.16.1.tar.gz"),
		cty.StringVal("tar -xzf v0.16.1.tar.gz"),
		cty.StringVal("cd wasmvm-0.16.1/"),
		cty.StringVal("sudo cp ./api/libwasmvm.so /usr/lib/"),
		cty.StringVal("cd .."),
		cty.StringVal("rm -r v0.16.1.tar.gz wasmvm-0.16.1"),
		cty.StringVal("systemctl restart systemd-journald"),
	}))

	connectionBlock := remoteExecBody.AppendNewBlock("connection", nil)
	connectionBody := connectionBlock.Body()
	connectionBody.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	connectionBody.SetAttributeValue("type", cty.StringVal("ssh"))
	connectionBody.SetAttributeValue("user", cty.StringVal("root"))
	key := hclwrite.Tokens{
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("private_key "),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenEqual,
			Bytes:        []byte("="),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(" file"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenOParen,
			Bytes:        []byte("("),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(fmt.Sprintf("\"%v\"", do.PriKeyPath)),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCParen,
			Bytes:        []byte(")"),
			SpacesBefore: 0,
		},
	}
	connectionBody.AppendUnstructuredTokens(key)
	connectionBody.AppendNewline()

	serviceFileBlock := dropletBody.AppendNewBlock("provisioner", []string{"file"})
	serviceFileBody := serviceFileBlock.Body()
	serviceFileBody.SetAttributeValue("source", cty.StringVal(do.ServiceFile))
	serviceFileBody.SetAttributeValue("destination", cty.StringVal("/home/darknode/darknode.service"))
	connection3Block := serviceFileBody.AppendNewBlock("connection", nil)
	connection3Body := connection3Block.Body()
	connection3Body.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	connection3Body.SetAttributeValue("type", cty.StringVal("ssh"))
	connection3Body.SetAttributeValue("user", cty.StringVal("darknode"))
	connection3Body.AppendUnstructuredTokens(key)
	connection3Body.AppendNewline()

	updaterServiceFileBlock := dropletBody.AppendNewBlock("provisioner", []string{"file"})
	updaterServiceFileBody := updaterServiceFileBlock.Body()
	updaterServiceFileBody.SetAttributeValue("source", cty.StringVal(do.UpdaterServiceFile))
	updaterServiceFileBody.SetAttributeValue("destination", cty.StringVal("/home/darknode/darknode-updater.service"))
	connection4Block := updaterServiceFileBody.AppendNewBlock("connection", nil)
	connection4Body := connection4Block.Body()
	connection4Body.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	connection4Body.SetAttributeValue("type", cty.StringVal("ssh"))
	connection4Body.SetAttributeValue("user", cty.StringVal("darknode"))
	connection4Body.AppendUnstructuredTokens(key)
	connection4Body.AppendNewline()

	snapshotURL := util.SnapshotURL(do.Network, "")
	remoteExec2Block := dropletBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExec2Body := remoteExec2Block.Body()
	remoteExec2Body.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("mkdir -p $HOME/.darknode/bin"),
		cty.StringVal("mkdir -p $HOME/.config/systemd/user"),
		cty.StringVal(fmt.Sprintf("cd .darknode && curl -sSOJL %v && tar xzf latest.tar.gz", snapshotURL)),
		cty.StringVal("rm latest.tar.gz"),
		cty.StringVal("mv $HOME/darknode.service $HOME/.config/systemd/user/darknode.service"),
		cty.StringVal("mv $HOME/darknode-updater.service $HOME/.config/systemd/user/darknode-updater.service"),
		cty.StringVal(fmt.Sprintf("curl -sL https://github.com/renproject/darknode-release/releases/download/%v/darknode > ~/.darknode/bin/darknode", do.Version)),
		cty.StringVal("curl -sL https://github.com/renproject/nodectl/releases/latest/download/darknode-updater > ~/.darknode/bin/darknode-updater"),
		cty.StringVal("chmod +x ~/.darknode/bin/darknode"),
		cty.StringVal("chmod +x ~/.darknode/bin/darknode-updater"),
		cty.StringVal("loginctl enable-linger darknode"),
		cty.StringVal("systemctl --user enable darknode.service"),
		cty.StringVal("systemctl --user enable darknode-updater.service"),
		cty.StringVal("systemctl --user start darknode-updater"),
		cty.StringVal(fmt.Sprintf("echo 'DARKNODE_SNAPSHOT_VERSIONID=%v' >> .env", do.SnapshotVersionID)),
		cty.StringVal(fmt.Sprintf("echo 'DARKNODE_CONFIG_VERSIONID=%v' >> .env", do.ConfigVersionID)),
		cty.StringVal(fmt.Sprintf("echo 'DARKNODE_INSTALLED=%v' >> .env", do.Version)),
		cty.StringVal("echo 'UPDATE_BIN=1' >> .env"),
		cty.StringVal("echo 'UPDATE_CONFIG=1' >> .env"),
		cty.StringVal("echo 'UPDATE_RECOVERY=1' >> .env"),
	}))

	connection5Block := remoteExec2Body.AppendNewBlock("connection", nil)
	connection5Body := connection5Block.Body()
	connection5Body.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	connection5Body.SetAttributeValue("type", cty.StringVal("ssh"))
	connection5Body.SetAttributeValue("user", cty.StringVal("darknode"))
	connection5Body.AppendUnstructuredTokens(key)
	connection5Body.AppendNewline()

	outputProviderBlock := rootBody.AppendNewBlock("output", []string{"provider"})
	outputProviderBody := outputProviderBlock.Body()
	outputProviderBody.SetAttributeValue("value", cty.StringVal("do"))

	outputIPBlock := rootBody.AppendNewBlock("output", []string{"ip"})
	outputIPBody := outputIPBlock.Body()
	outputIPBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "digitalocean_droplet",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})

	return f.Bytes()
}
