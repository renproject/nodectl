package provider2

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
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
	filterBody.SetAttributeValue("values", cty.ListVal([]cty.Value{cty.StringVal("ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-*")}))

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
	ingressRenBody.SetAttributeValue("to_port", cty.NumberIntVal(18514))
	ingressRenBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ingressRenBody.SetAttributeValue("cidr_blocks", cty.ListVal([]cty.Value{cty.StringVal("0.0.0.0/0")}))

	egressBlock := sgBody.AppendNewBlock("egress", nil)
	egressBody := egressBlock.Body()
	egressBody.SetAttributeValue("from_port", cty.NumberIntVal(0))
	egressBody.SetAttributeValue("to_port", cty.NumberIntVal(0))
	egressBody.SetAttributeValue("protocol", cty.StringVal("-1"))
	egressBody.SetAttributeValue("cidr_blocks", cty.ListVal([]cty.Value{cty.StringVal("0.0.0.0/0")}))

	keypairBlock :=  rootBody.AppendNewBlock("aws_key_pair", []string{"darknode"})
	keypairBody := keypairBlock.Body()
	keypairBody.SetAttributeValue("key_name", cty.StringVal(aws.Name))
	keypairBody.SetAttributeValue("public_key", cty.TupleVal([]cty.Value{cty.StringVal(aws.PubKeyPath)}))

	instanceBlock :=  rootBody.AppendNewBlock("resource", []string{"aws_instance", "darknode"})
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
	instanceBody.SetAttributeTraversal("ami", hcl.Traversal{
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
		cty.StringVal( "sudo rsync --archive --chown=darknode:darknode ~/.ssh /home/darknode"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y update"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y upgrade"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y dist-upgrade"),
		cty.StringVal("sudo DEBIAN_FRONTEND=noninteractive apt-get -y autoremove"),
		cty.StringVal( "sudo apt-get install ufw"),
		cty.StringVal("sudo ufw limit 22/tcp"),
		cty.StringVal("sudo ufw allow 18514/tcp"),
		cty.StringVal("sudo ufw allow 18515/tcp"),
		cty.StringVal("sudo ufw --force enable"),
	}))
	
	remoteConnectionBlock := remoteExecBody.AppendNewBlock("connection", nil)
	remoteConnectionBody := remoteConnectionBlock.Body()
	//TODO function host
	remoteConnectionBody.SetAttributeValue("type", cty.StringVal("ssh"))
	remoteConnectionBody.SetAttributeValue("user", cty.StringVal("ubuntu"))
	//TODO function private_key
	
	configBlock := instanceBody.AppendNewBlock("provisioner", []string{"file"})
	configBody := configBlock.Body()
	configBody.SetAttributeValue("source", cty.StringVal(aws.ConfigPath))
	configBody.SetAttributeValue("destination", cty.StringVal("$HOME/config.json"))
	
	configConnBlock := configBody.AppendNewBlock("connection", nil)
	configConnBody := configConnBlock.Body()
	//TODO function host
	configConnBody.SetAttributeValue("Type", cty.StringVal("ssh"))
	configConnBody.SetAttributeValue("user", cty.StringVal("darknode"))
	//TODO function private_key

	remoteExec2Block := instanceBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExec2Body := remoteExec2Block.Body()
	remoteExec2Body.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("mkdir -p $HOME/.darknode/bin"),
		cty.StringVal("mkdir -p $HOME/.config/systemd/user"),
		cty.StringVal( "mv $HOME/config.json $HOME/.darknode/config.json"),
		cty.StringVal("curl -sL https://www.github.com/renproject/darknode-release/releases/latest/download/darknode > ~/.darknode/bin/darknode"),
		cty.StringVal("chmod +x ~/.darknode/bin/darknode"),
		cty.StringVal("echo {{.LatestVersion}} > ~/.darknode/version"),
		//TODO add EOT and writing service file
		cty.StringVal("loginctl enable-linger darknode"),
		cty.StringVal( "systemctl --user enable darknode.service"),
		cty.StringVal("systemctl --user start darknode.service"),
	}))

	remoteConnection2Block := remoteExecBody.AppendNewBlock("connection", nil)
	remoteConnection2Body := remoteConnection2Block.Body()
	//TODO function host
	remoteConnection2Body.SetAttributeValue("type", cty.StringVal("ssh"))
	remoteConnection2Body.SetAttributeValue("user", cty.StringVal("darknode"))
	//TODO function private_key

	outputProviderBlock := rootBody.AppendNewBlock("output", []string{"output", "provider"})
	outputProviderBody := outputProviderBlock.Body()
	outputProviderBody.SetAttributeValue("value", cty.StringVal("aws"))


	outputIPBlock := rootBody.AppendNewBlock("output", []string{"output", "provider"})
	outputIPBody := outputIPBlock.Body()
	outputIPBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_instancer",
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