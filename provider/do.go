package provider

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
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
	name := ctx.String("name")
	if err := validateCommonParams(ctx); err != nil {
		return err
	}
	region, droplet, err := p.validateRegionAndDroplet(ctx)
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

	log.Printf("name = %v, region = %v, droplet = %v, version = %v", name, region, droplet, version)

	return nil
}

//
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
		if droplet == DefaultDigitalOceanDroplet {
			return regions[rand.Intn(len(regions))], droplet, nil
		} else {
			// Randomly select a region which has the given droplet size.
			indexes := rand.Perm(len(regions))
			for _, index := range indexes {
				if util.StringInSlice(droplet, regions[index].Sizes) {
					return regions[index], droplet, nil
				}
			}
			return godo.Region{}, "", fmt.Errorf("selected droplet [%v] not available across all regions", droplet)
		}
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
