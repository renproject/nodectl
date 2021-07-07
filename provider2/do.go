package provider2

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type doTerraform struct {
	Name          string
	Token         string
	Region        string
	Size          string
	ConfigPath    string
	PubKeyPath    string
	PriKeyPath    string
	ServiceFile   string
	LatestVersion string
}

func (do doTerraform) GenerateStaticIPConfig() {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

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
	fmt.Printf("%s\n", f.Bytes())
}

func (do doTerraform) GenerateTerraformConfig() {
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

	fmt.Printf("%s\n", f.Bytes())
}
