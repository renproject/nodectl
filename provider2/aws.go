package provider2

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type terraformAWS struct {
	Name          string
	Region        string
	InstanceType  string
	ConfigPath    string
	PubKeyPath    string
	PriKeyPath    string
	AccessKey     string
	SecretKey     string
	ServiceFile   string
	LatestVersion string
}

func (aws terraformAWS) GenerateStaticIPConfig() {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	rootBody.AppendNewBlock("resource", []string{"aws_eip", "darknode"})

	outputIPBlock := rootBody.AppendNewBlock("output", []string{"static_ip"})
	outputIPBody := outputIPBlock.Body()
	outputIPBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_eip",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "public_ip",
		},
	})
	fmt.Printf("%s\n", f.Bytes())
}

func (aws terraformAWS) GenerateTerraformConfig() {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	providerBlock := rootBody.AppendNewBlock("provider", []string{"aws"})
	providerBody := providerBlock.Body()
	providerBody.SetAttributeValue("region", cty.StringVal(aws.Region))
	providerBody.SetAttributeValue("access_key", cty.StringVal(aws.AccessKey))
	providerBody.SetAttributeValue("secret_key", cty.StringVal(aws.SecretKey))

	imageBlock := rootBody.AppendNewBlock("data", []string{"aws_ami", "ubuntu"})
	imageBody := imageBlock.Body()
	imageBody.SetAttributeValue("most_recent", cty.True)

	filterBlock := imageBody.AppendNewBlock("filter", nil)
	filterBody := filterBlock.Body()
	filterBody.SetAttributeValue("name", cty.StringVal("name"))
	filterBody.SetAttributeValue("values", cty.ListVal([]cty.Value{cty.StringVal("ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*")}))

	filter2Block := imageBody.AppendNewBlock("filter", nil)
	filter2Body := filter2Block.Body()
	filter2Body.SetAttributeValue("name", cty.StringVal("virtualization-type"))
	filter2Body.SetAttributeValue("values", cty.ListVal([]cty.Value{cty.StringVal("hvm")}))

	imageBody.SetAttributeValue("owners", cty.ListVal([]cty.Value{cty.StringVal("099720109477")}))

	sgBlock := rootBody.AppendNewBlock("resource", []string{"aws_security_group", "darknode"})
	sgBody := sgBlock.Body()
	sgBody.SetAttributeValue("name", cty.StringVal(fmt.Sprintf("darknode-sg-%v", aws.Name)))
	sgBody.SetAttributeValue("description", cty.StringVal("Allow inbound SSH and REN project traffic"))

	ingressSSHBlock := sgBody.AppendNewBlock("ingress", nil)
	ingressSSHBody := ingressSSHBlock.Body()
	ingressSSHBody.SetAttributeValue("from_port", cty.NumberIntVal(22))
	ingressSSHBody.SetAttributeValue("to_port", cty.NumberIntVal(22))
	ingressSSHBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ingressSSHBody.SetAttributeValue("cidr_blocks", cty.ListVal([]cty.Value{cty.StringVal("0.0.0.0/0")}))

	ingressRenBlock := sgBody.AppendNewBlock("ingress", nil)
	ingressRenBody := ingressRenBlock.Body()
	ingressRenBody.SetAttributeValue("from_port", cty.NumberIntVal(18514))
	ingressRenBody.SetAttributeValue("to_port", cty.NumberIntVal(18515))
	ingressRenBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ingressRenBody.SetAttributeValue("cidr_blocks", cty.ListVal([]cty.Value{cty.StringVal("0.0.0.0/0")}))

	egressBlock := sgBody.AppendNewBlock("egress", nil)
	egressBody := egressBlock.Body()
	egressBody.SetAttributeValue("from_port", cty.NumberIntVal(0))
	egressBody.SetAttributeValue("to_port", cty.NumberIntVal(0))
	egressBody.SetAttributeValue("protocol", cty.StringVal("-1"))
	egressBody.SetAttributeValue("cidr_blocks", cty.ListVal([]cty.Value{cty.StringVal("0.0.0.0/0")}))

	keypairBlock := rootBody.AppendNewBlock("resource", []string{"aws_key_pair", "darknode"})
	keypairBody := keypairBlock.Body()
	keypairBody.SetAttributeValue("key_name", cty.StringVal(aws.Name))
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
			Bytes:        []byte(fmt.Sprintf("\"%v\"", aws.PubKeyPath)),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCParen,
			Bytes:        []byte(")"),
			SpacesBefore: 0,
		},
	}
	keypairBody.AppendUnstructuredTokens(pubKey)
	keypairBody.AppendNewline()

	instanceBlock := rootBody.AppendNewBlock("resource", []string{"aws_instance", "darknode"})
	instanceBody := instanceBlock.Body()
	instanceBody.SetAttributeTraversal("ami", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "data",
		},
		hcl.TraverseAttr{
			Name: "aws_ami",
		},
		hcl.TraverseAttr{
			Name: "ubuntu",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
	instanceBody.SetAttributeValue("instance_type", cty.StringVal(aws.InstanceType))
	instanceBody.SetAttributeTraversal("key_name", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_key_pair",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "key_name",
		},
	})

	securityGroups := hclwrite.Tokens{
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("security_groups "),
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
			Bytes:        []byte("aws_security_group.darknode.name"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCBrack,
			Bytes:        []byte("]"),
			SpacesBefore: 0,
		},
	}
	instanceBody.AppendUnstructuredTokens(securityGroups)
	instanceBody.AppendNewline()
	instanceBody.SetAttributeValue("monitoring", cty.True)
	instanceBody.SetAttributeValue("tags", cty.ObjectVal(map[string]cty.Value{
		"Name": cty.StringVal(aws.Name),
	}))

	rootBlockDevice := instanceBody.AppendNewBlock("root_block_device", nil)
	rootBlockDeviceBody := rootBlockDevice.Body()
	rootBlockDeviceBody.SetAttributeValue("volume_type", cty.StringVal("gp2"))
	rootBlockDeviceBody.SetAttributeValue("volume_size", cty.NumberIntVal(15))

	remoteExecBlock := instanceBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExecBody := remoteExecBlock.Body()
	remoteExecBody.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("until sudo apt update; do sleep 4; done"),
		cty.StringVal("sudo adduser darknode --gecos \",,,\" --disabled-password"),
		cty.StringVal("sudo rsync --archive --chown=darknode:darknode ~/.ssh /home/darknode"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y update"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y upgrade"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y dist-upgrade"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y autoremove"),
		cty.StringVal("sudo apt-get install -y ocl-icd-opencl-dev build-essential libhwloc-dev"),
		cty.StringVal("until sudo apt-get install -y ufw; do sleep 4; done"),
		cty.StringVal("sudo ufw limit 22/tcp"),
		cty.StringVal("sudo ufw allow 18514/tcp"),
		cty.StringVal("sudo ufw allow 18515/tcp"),
		cty.StringVal("sudo ufw --force enable"),
	}))

	remoteConnectionBlock := remoteExecBody.AppendNewBlock("connection", nil)
	remoteConnectionBody := remoteConnectionBlock.Body()
	host := hclwrite.Tokens{
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("host "),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenEqual,
			Bytes:        []byte("="),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte(" coalesce"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenOParen,
			Bytes:        []byte("("),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenStringLit,
			Bytes:        []byte("self.public_ip, self.private_ip"),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCParen,
			Bytes:        []byte(")"),
			SpacesBefore: 0,
		},
	}
	remoteConnectionBody.AppendUnstructuredTokens(host)
	remoteConnectionBody.AppendNewline()
	remoteConnectionBody.SetAttributeValue("type", cty.StringVal("ssh"))
	remoteConnectionBody.SetAttributeValue("user", cty.StringVal("ubuntu"))
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
			Bytes:        []byte(fmt.Sprintf("\"%v\"", aws.PriKeyPath)),
			SpacesBefore: 0,
		},
		&hclwrite.Token{
			Type:         hclsyntax.TokenCParen,
			Bytes:        []byte(")"),
			SpacesBefore: 0,
		},
	}
	remoteConnectionBody.AppendUnstructuredTokens(key)
	remoteConnectionBody.AppendNewline()

	configBlock := instanceBody.AppendNewBlock("provisioner", []string{"file"})
	configBody := configBlock.Body()
	configBody.SetAttributeValue("source", cty.StringVal(aws.ConfigPath))
	configBody.SetAttributeValue("destination", cty.StringVal("$HOME/config.json"))

	configConnBlock := configBody.AppendNewBlock("connection", nil)
	configConnBody := configConnBlock.Body()
	configConnBody.AppendUnstructuredTokens(host)
	configConnBody.AppendNewline()
	configConnBody.SetAttributeValue("type", cty.StringVal("ssh"))
	configConnBody.SetAttributeValue("user", cty.StringVal("darknode"))
	configConnBody.AppendUnstructuredTokens(key)
	configConnBody.AppendNewline()

	serviceFileBlock := instanceBody.AppendNewBlock("provisioner", []string{"file"})
	serviceFileBody := serviceFileBlock.Body()
	serviceFileBody.SetAttributeValue("source", cty.StringVal("../artifacts/darknode.service"))
	serviceFileBody.SetAttributeValue("destination", cty.StringVal("~/.config/systemd/user/darknode.service"))
	serviceConnectionBlock := serviceFileBody.AppendNewBlock("connection", nil)
	serviceConnectionBody := serviceConnectionBlock.Body()
	serviceConnectionBody.SetAttributeTraversal("host", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "self",
		},
		hcl.TraverseAttr{
			Name: "ipv4_address",
		},
	})
	serviceConnectionBody.SetAttributeValue("type", cty.StringVal("ssh"))
	serviceConnectionBody.SetAttributeValue("user", cty.StringVal("root"))
	serviceConnectionBody.AppendUnstructuredTokens(key)
	serviceConnectionBody.AppendNewline()

	remoteExec2Block := instanceBody.AppendNewBlock("provisioner", []string{"remote-exec"})
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
		cty.StringVal("mkdir -p $HOME/.config/systemd/user"),
		cty.StringVal("mv $HOME/config.json $HOME/.darknode/config.json"),
		cty.StringVal("curl -sL https://www.github.com/renproject/darknode-release/releases/latest/download/darknode > ~/.darknode/bin/darknode"),
		cty.StringVal("chmod +x ~/.darknode/bin/darknode"),
		cty.StringVal("echo {{.LatestVersion}} > ~/.darknode/version"),
		cty.StringVal("loginctl enable-linger darknode"),
		cty.StringVal("systemctl --user enable darknode.service"),
		cty.StringVal("systemctl --user start darknode.service"),
	}))

	remoteConnection2Block := remoteExec2Body.AppendNewBlock("connection", nil)
	remoteConnection2Body := remoteConnection2Block.Body()
	remoteConnection2Body.AppendUnstructuredTokens(host)
	remoteConnection2Body.AppendNewline()
	remoteConnection2Body.SetAttributeValue("type", cty.StringVal("ssh"))
	remoteConnection2Body.SetAttributeValue("user", cty.StringVal("darknode"))
	remoteConnection2Body.AppendUnstructuredTokens(key)
	remoteConnection2Body.AppendNewline()

	outputProviderBlock := rootBody.AppendNewBlock("output", []string{"provider"})
	outputProviderBody := outputProviderBlock.Body()
	outputProviderBody.SetAttributeValue("value", cty.StringVal("aws"))

	outputIPBlock := rootBody.AppendNewBlock("output", []string{"ip"})
	outputIPBody := outputIPBlock.Body()
	outputIPBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_instance",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "public_ip",
		},
	})

	fmt.Printf("%s\n", f.Bytes())
}