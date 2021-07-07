package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/renproject/aw/wire"
	"github.com/renproject/multichain"
	"github.com/renproject/nodectl/renvm"
	"github.com/zclconf/go-cty/cty"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
)

const DefaultDigitalOceanDroplet = "s-1vcpu-1gb"
const DefaultDigitalOceanDropletSize = "15"

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
	name := ctx.String("name")
	if err := validateCommonParams(ctx); err != nil {
		return err
	}
	network := multichain.Network(ctx.String("network"))
	region, droplet, err := p.validateRegionAndDroplet(ctx)
	if err != nil {
		return err
	}
	version, err := util.LatestStableRelease()
	if err != nil {
		return err
	}

	// Fetch the remote config template
	configURL := renvm.ConfigURLTestnet
	if network == multichain.NetworkMainnet {
		configURL = renvm.ConfigURLMainnet
	}
	templateOpts, err := renvm.OptionTemplate(configURL)
	if err != nil {
		return err
	}

	// Initialize folder and files for the node
	if err := initialize(ctx, templateOpts); err != nil {
		return err
	}

	tf := doTerraform{
		Name:          name,
		Token:         p.token,
		Region:        region.Name,
		Size:          DefaultDigitalOceanDropletSize,
		ConfigPath:     "",
		GenesisPath:   filepath.Join(util.NodePath(name), "genesis.json"),
		PubKeyPath:    filepath.Join(util.NodePath(name), "ssh_keypair.pub"),
		PriKeyPath:    filepath.Join(util.NodePath(name), "ssh_keypair"),
		ServiceFile:    filepath.Join(util.NodePath(name), "darknode.service"),
		LatestVersion: version,
	}

	// TODO: Apply the terraform config
	color.Green("Creating a static IP address for darknode...")
	fipData := tf.GenerateStaticIPConfig()
	fipFile, err := os.Create(filepath.Join(util.NodePath(name), "ip.tf"))
	if err != nil {
		return err
	}
	if _, err := fipFile.Write(fipData); err != nil {
		return err
	}
	fipFile.Close()
	if err := applyTerraform(name); err != nil {
		return err
	}

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
	opts.Peers = append(templateOpts.Peers, addr)
	opts.Selectors = templateOpts.Selectors
	opts.Chains = templateOpts.Chains
	configPath := filepath.Join(util.NodePath(name), "config.json")
	configFile, err := os.Create(configPath)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(opts); err != nil {
		return err
	}
	configFile.Close()
	tf.ConfigPath = configPath

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
	color.Green("Your darknode is up and running")
	log.Printf("name = %v, region = %v, droplet = %v, version = %v", name, region, droplet, version)
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
				return regions[index], droplet, nil
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
	Name          string
	Token         string
	Region        string
	Size          string
	ConfigPath     string
	GenesisPath   string
	PubKeyPath    string
	PriKeyPath    string
	ServiceFile   string
	LatestVersion string
}

func (do doTerraform) GenerateStaticIPConfig() []byte {
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

	floatingIpBlock := rootBody.AppendNewBlock("resource", []string{"digitalocean_floating_ip", "darknode"})
	floatingIpBody := floatingIpBlock.Body()
	floatingIpBody.SetAttributeValue("region", cty.StringVal(do.Region))

	outputIPBlock := rootBody.AppendNewBlock("output", []string{"static_ip"})
	outputIPBody := outputIPBlock.Body()
	outputIPBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "digitalocean_floating_ip",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "ip_address",
		},
	})
	return f.Bytes()
}

func (do doTerraform) GenerateTerraformConfig() []byte {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

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
		cty.StringVal("sudo apt-get install -y ocl-icd-opencl-dev build-essential libhwloc-dev"),
		cty.StringVal("until sudo apt-get install -y ufw; do sleep 4; done"),
		cty.StringVal("sudo ufw limit 22/tcp"),
		cty.StringVal("sudo ufw allow 18514/tcp"),
		cty.StringVal("sudo ufw allow 18515/tcp"),
		cty.StringVal("sudo ufw --force enable"),
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

	configFileBlock := dropletBody.AppendNewBlock("provisioner", []string{"file"})
	configFileBody := configFileBlock.Body()
	configFileBody.SetAttributeValue("source", cty.StringVal(do.ConfigPath))
	configFileBody.SetAttributeValue("destination", cty.StringVal("$HOME/config.json"))
	connection2Block := configFileBody.AppendNewBlock("connection", nil)
	connection2Body := connection2Block.Body()
	connection2Body.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	connection2Body.SetAttributeValue("type", cty.StringVal("ssh"))
	connection2Body.SetAttributeValue("user", cty.StringVal("root"))
	connection2Body.AppendUnstructuredTokens(key)
	connection2Body.AppendNewline()

	genesisBlock := dropletBody.AppendNewBlock("provisioner", []string{"file"})
	genesisBody := genesisBlock.Body()
	genesisBody.SetAttributeValue("source", cty.StringVal(do.GenesisPath))
	genesisBody.SetAttributeValue("destination", cty.StringVal("$HOME/genesis.json"))
	genesisConnBlock := genesisBody.AppendNewBlock("connection", nil)
	genesisConnBody := genesisConnBlock.Body()
	genesisConnBody.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	genesisConnBody.SetAttributeValue("type", cty.StringVal("ssh"))
	genesisConnBody.SetAttributeValue("user", cty.StringVal("darknode"))
	genesisConnBody.AppendUnstructuredTokens(key)
	genesisConnBody.AppendNewline()

	serviceFileBlock := dropletBody.AppendNewBlock("provisioner", []string{"file"})
	serviceFileBody := serviceFileBlock.Body()
	serviceFileBody.SetAttributeValue("source", cty.StringVal("../artifacts/darknode.service"))
	serviceFileBody.SetAttributeValue("destination", cty.StringVal("~/.config/systemd/user/darknode.service"))
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
	connection3Body.SetAttributeValue("user", cty.StringVal("root"))
	connection3Body.AppendUnstructuredTokens(key)
	connection3Body.AppendNewline()

	remoteExec2Block := dropletBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExec2Body := remoteExec2Block.Body()
	remoteExec2Body.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("curl https://sh.rustup.rs -sSf | sh"),
		cty.StringVal("source $HOME/.cargo/env"),
		cty.StringVal("wget https://github.com/CosmWasm/wasmvm/archive/v0.10.0.tar.gz"),
		cty.StringVal("tar -xzf v0.10.0.tar.gz"),
		cty.StringVal("cd wasmvm-0.10.0/"),
		cty.StringVal("make build"),
		cty.StringVal("sudo cp ./api/libgo_cosmwasm.so /usr/lib/"),
		cty.StringVal("cd .."),
		cty.StringVal("rm -r v0.10.0.tar.gz wasmvm-0.10.0"),
		cty.StringVal("mkdir -p $HOME/.darknode/bin"),
		cty.StringVal("mkdir -p $HOME/.config/systemd/use"),
		cty.StringVal("mv $HOME/config.json $HOME/.darknode/config.json"),
		cty.StringVal(fmt.Sprintf("curl -sL https://www.github.com/renproject/darknode-release/releases/download/%v/darknode > ~/.darknode/bin/darknode", do.LatestVersion)),
		cty.StringVal("chmod +x ~/.darknode/bin/darknod"),
		cty.StringVal(fmt.Sprintf("echo %s > ~/.darknode/version", do.LatestVersion)),
		cty.StringVal("loginctl enable-linger darknode"),
		cty.StringVal("systemctl --user enable darknode.service"),
		cty.StringVal("systemctl --user start darknode.service"),
	}))

	connection4Block := remoteExec2Body.AppendNewBlock("connection", nil)
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
	connection4Body.SetAttributeValue("user", cty.StringVal("root"))
	connection4Body.AppendUnstructuredTokens(key)
	connection4Body.AppendNewline()

	//resource "digitalocean_floating_ip_assignment" "foobar" {
	//	ip_address = digitalocean_floating_ip.foobar.ip_address
	//	droplet_id = digitalocean_droplet.foobar.id
	//}
	floatingIPBlock := rootBody.AppendNewBlock("resource", []string{"digitalocean_floating_ip_assignment", "foobar"})
	floatingIPBody := floatingIPBlock.Body()
	floatingIPBody.SetAttributeTraversal("ip_address", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "digitalocean_floating_ip",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "ip_address",
		},
	})
	floatingIPBody.SetAttributeTraversal("droplet_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "digitalocean_droplet",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})

	outputProviderBlock := rootBody.AppendNewBlock("output", []string{"provider"})
	outputProviderBody := outputProviderBlock.Body()
	outputProviderBody.SetAttributeValue("value", cty.StringVal("do"))

	return f.Bytes()
}
