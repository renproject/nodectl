package provider

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
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
	name := ctx.String("name")
	if err := validateCommonParams(ctx); err != nil {
		return err
	}
	region, instance, err := p.validateRegionAndInstance(ctx)
	if err != nil {
		return err
	}
	version, err := util.LatestStableRelease()
	if err != nil {
		return err
	}

	// Initialize folder and files for the node
	if err := initialize(ctx); err != nil {
		return err
	}

	// TODO: Generate a terraform config file

	// TODO: Apply the terraform config

	// TODO: Redirect them to register the node

	log.Printf("name = %v, region = %v, instance = %v, version = %v", name, region, instance, version)

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
			if p.instanceTypesAvailability(cred, region, instance); err == nil {
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
