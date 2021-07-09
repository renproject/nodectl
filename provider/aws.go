package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
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

const DefaultAWSInstance = "t3.micro"

type providerAWS struct {
	accessKey string
	secretKey string
}

// NewAWS creates a AWS provider.
func NewAWS(ctx *cli.Context) (Provider, error) {
	accessKey := ctx.String("aws-access-key")
	secretKey := ctx.String("aws-secret-key")

	// Try reading the default credential file if user does not provide credentials directly
	if accessKey == "" || secretKey == "" {
		cred := credentials.NewSharedCredentials("", ctx.String("aws-profile"))
		credValue, err := cred.Get()
		if err != nil {
			return nil, errors.New("invalid credentials")
		}
		accessKey, secretKey = credValue.AccessKeyID, credValue.SecretAccessKey
		if accessKey == "" || secretKey == "" {
			return nil, errors.New("invalid credentials")
		}
	}

	return providerAWS{
		accessKey: accessKey,
		secretKey: secretKey,
	}, nil
}

// Name implements the `Provider` interface
func (p providerAWS) Name() string {
	return NameAws
}

// Deploy implements the `Provider` interface
func (p providerAWS) Deploy(ctx *cli.Context) error {
	// Validate all input params
	if err := validateCommonParams(ctx); err != nil {
		return err
	}
	name := ctx.String("name")
	network := multichain.Network(ctx.String("network"))
	region, instance, err := p.validateRegionAndInstance(ctx)
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

	// Getting everything needed by terraform
	tf := terraformAWS{
		Name:         name,
		Region:       region,
		InstanceType: instance,
		ConfigPath:   "", // Will be filled after generating the EIP
		GenesisPath:  filepath.Join(util.NodePath(name), "genesis.json"),
		PubKeyPath:   filepath.Join(util.NodePath(name), "ssh_keypair.pub"),
		PriKeyPath:   filepath.Join(util.NodePath(name), "ssh_keypair"),
		AccessKey:    p.accessKey,
		SecretKey:    p.secretKey,
		ServiceFile:  filepath.Join(util.NodePath(name), "darknode.service"),
	}

	// Create the EIP
	color.Green("Creating a static IP address for darknode...")
	eipData := tf.GenerateStaticIPConfig()
	eipFile, err := os.Create(filepath.Join(util.NodePath(name), "ip.tf"))
	if err != nil {
		return err
	}
	if _, err := eipFile.Write(eipData); err != nil {
		return err
	}
	eipFile.Close()
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

	// Create the rest service on the cloud
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
	return nil
}

func (p providerAWS) validateRegionAndInstance(ctx *cli.Context) (string, string, error) {
	cred := credentials.NewStaticCredentials(p.accessKey, p.secretKey, "")
	region := strings.ToLower(strings.TrimSpace(ctx.String("aws-region")))
	instance := strings.ToLower(strings.TrimSpace(ctx.String("aws-instance")))

	// Get all available regions
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Credentials: cred,
	})
	service := ec2.New(sess)
	input := &ec2.DescribeRegionsInput{}
	result, err := service.DescribeRegions(input)
	if err != nil {
		return "", "", err
	}
	regions := make([]string, len(result.Regions))
	for i := range result.Regions {
		regions[i] = *result.Regions[i].RegionName
	}

	if region == "" {
		// Randomly select a region which has the given droplet size.
		indexes := rand.Perm(len(result.Regions))
		for _, index := range indexes {
			region = *result.Regions[index].RegionName
			if err := p.instanceTypesAvailability(cred, region, instance); err == nil {
				return region, instance, nil
			}
		}
		return "", "", fmt.Errorf("selected instance type [%v] is not available across all regions", instance)
	} else {
		err = p.instanceTypesAvailability(cred, region, instance)
		return region, instance, err
	}
}

func (p providerAWS) instanceTypesAvailability(cred *credentials.Credentials, region, instance string) error {
	instanceSession, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: cred,
	})
	if err != nil {
		return err
	}
	service := ec2.New(instanceSession)
	instanceInput := &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []*string{aws.String(instance)},
	}
	instanceResult, err := service.DescribeInstanceTypes(instanceInput)
	if err != nil {
		return err
	}
	for _, res := range instanceResult.InstanceTypes {
		if *res.InstanceType == instance {
			return nil
		}
	}
	return fmt.Errorf("instance not avaliable")
}

type terraformAWS struct {
	Name         string
	Region       string
	InstanceType string
	ConfigPath   string
	GenesisPath  string
	PubKeyPath   string
	PriKeyPath   string
	AccessKey    string
	SecretKey    string
	ServiceFile  string
}

func (aws terraformAWS) GenerateStaticIPConfig() []byte {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	providerBlock := rootBody.AppendNewBlock("provider", []string{"aws"})
	providerBody := providerBlock.Body()
	providerBody.SetAttributeValue("region", cty.StringVal(aws.Region))
	providerBody.SetAttributeValue("access_key", cty.StringVal(aws.AccessKey))
	providerBody.SetAttributeValue("secret_key", cty.StringVal(aws.SecretKey))

	rootBody.AppendNewBlock("resource", []string{"aws_eip", "darknode"})

	outputIPBlock := rootBody.AppendNewBlock("output", []string{"ip"})
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
	return f.Bytes()
}

func (aws terraformAWS) GenerateTerraformConfig() []byte {
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	eipaBlock := rootBody.AppendNewBlock("resource", []string{"aws_eip_association", "darknode"})
	eipaBody := eipaBlock.Body()
	eipaBody.SetAttributeTraversal("instance_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_instance",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
	eipaBody.SetAttributeTraversal("allocation_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_eip",
		},
		hcl.TraverseAttr{
			Name: "darknode",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})

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
		cty.StringVal("until sudo apt-get install -y ufw; do sleep 4; done"),
		cty.StringVal("sudo ufw limit 22/tcp"),
		cty.StringVal("sudo ufw allow 18514/tcp"),
		cty.StringVal("sudo ufw allow 18515/tcp"),
		cty.StringVal("sudo ufw --force enable"),
		cty.StringVal("sudo apt-get install -y ocl-icd-opencl-dev build-essential libhwloc-dev"),
		cty.StringVal("curl https://sh.rustup.rs -sSf | sh -s -- -y"),
		cty.StringVal("wget https://github.com/CosmWasm/wasmvm/archive/v0.10.0.tar.gz"),
		cty.StringVal("tar -xzf v0.10.0.tar.gz"),
		cty.StringVal("cd wasmvm-0.10.0/"),
		cty.StringVal("sudo cp ./api/libgo_cosmwasm.so /usr/lib/"),
		cty.StringVal("cd .."),
		cty.StringVal("rm -r v0.10.0.tar.gz wasmvm-0.10.0"),
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

	genesisBlock := instanceBody.AppendNewBlock("provisioner", []string{"file"})
	genesisBody := genesisBlock.Body()
	genesisBody.SetAttributeValue("source", cty.StringVal(aws.GenesisPath))
	genesisBody.SetAttributeValue("destination", cty.StringVal("$HOME/genesis.json"))
	genesisConnBlock := genesisBody.AppendNewBlock("connection", nil)
	genesisConnBody := genesisConnBlock.Body()
	genesisConnBody.AppendUnstructuredTokens(host)
	genesisConnBody.AppendNewline()
	genesisConnBody.SetAttributeValue("type", cty.StringVal("ssh"))
	genesisConnBody.SetAttributeValue("user", cty.StringVal("darknode"))
	genesisConnBody.AppendUnstructuredTokens(key)
	genesisConnBody.AppendNewline()

	serviceFileBlock := instanceBody.AppendNewBlock("provisioner", []string{"file"})
	serviceFileBody := serviceFileBlock.Body()
	serviceFileBody.SetAttributeValue("source", cty.StringVal(aws.ServiceFile))
	serviceFileBody.SetAttributeValue("destination", cty.StringVal("$HOME/darknode.service"))
	serviceConnectionBlock := serviceFileBody.AppendNewBlock("connection", nil)
	serviceConnectionBody := serviceConnectionBlock.Body()
	serviceConnectionBody.AppendUnstructuredTokens(host)
	serviceConnectionBody.AppendNewline()
	serviceConnectionBody.SetAttributeValue("type", cty.StringVal("ssh"))
	serviceConnectionBody.SetAttributeValue("user", cty.StringVal("darknode"))
	serviceConnectionBody.AppendUnstructuredTokens(key)
	serviceConnectionBody.AppendNewline()

	remoteExec2Block := instanceBody.AppendNewBlock("provisioner", []string{"remote-exec"})
	remoteExec2Body := remoteExec2Block.Body()
	remoteExec2Body.SetAttributeValue("inline", cty.ListVal([]cty.Value{
		cty.StringVal("set -x"),
		cty.StringVal("mkdir -p $HOME/.darknode/bin"),
		cty.StringVal("mkdir -p $HOME/.config/systemd/user"),
		cty.StringVal("mv $HOME/config.json $HOME/.darknode/config.json"),
		cty.StringVal("mv $HOME/genesis.json $HOME/.darknode/genesis.json"),
		cty.StringVal("mv $HOME/darknode.service $HOME/.config/systemd/user/darknode.service"),
		// TODO : binary version
		// cty.StringVal("curl -sL https://www.github.com/renproject/darknode-release/releases/latest/download/darknode > ~/.darknode/bin/darknode"),
		cty.StringVal("curl -sL https://github.com/renproject/darknode-release/releases/download/0.4-mainnet15/darknode > ~/.darknode/bin/darknode"),
		cty.StringVal("chmod +x ~/.darknode/bin/darknode"),
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

	return f.Bytes()
}