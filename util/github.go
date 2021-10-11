package util

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v36/github"
	"github.com/hashicorp/go-version"
	"github.com/renproject/multichain"
	"golang.org/x/oauth2"
)

// GithubClient initialize the github client. If an access token has been set as an environment,
// it will use it for oauth to avoid rate limiting.
func GithubClient(ctx context.Context) *github.Client {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	var client *http.Client
	if accessToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: accessToken},
		)
		client = oauth2.NewClient(ctx, ts)
	}

	return github.NewClient(client)
}

// LatestStableRelease checks the node release repo and return the version of the latest release.
func LatestStableRelease() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	latest, err := version.NewVersion("0.0.0")
	if err != nil {
		return "", err
	}
	client := GithubClient(ctx)
	opts := &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	for {
		releases, response, err := client.Repositories.ListReleases(ctx, "renproject", "darknode-release", opts)
		if err != nil {
			return "", err
		}

		// Verify the status code is 200.
		if err := VerifyStatusCode(response.Response, http.StatusOK); err != nil {
			return "", err
		}

		// Find the latest stable production release
		stableVer, err := regexp.Compile("^v?[0-9]+\\.[0-9]+\\.[0-9]+$")
		if err != nil {
			return "", err
		}
		for _, release := range releases {
			if stableVer.MatchString(*release.TagName) {
				ver, err := version.NewVersion(*release.TagName)
				if err != nil {
					continue
				}
				if ver.GreaterThan(latest) {
					latest = ver
				}
			}
		}

		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	if latest.String() == "0.0.0" {
		return "", errors.New("cannot find any stable release")
	}

	return latest.String(), nil
}

func LatestRelease(network multichain.Network) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	max := 0
	tag := ""
	prefix := ""
	switch network {
	case multichain.NetworkMainnet:
		prefix = "0.4-mainnet"
	case multichain.NetworkTestnet:
		prefix = "0.4-testnet"
	case multichain.NetworkDevnet:
		prefix = "0.4-devnet"
	}

	client := GithubClient(ctx)
	opts := &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}
	for {
		releases, response, err := client.Repositories.ListReleases(ctx, "renproject", "darknode-release", opts)
		if err != nil {
			return "", err
		}

		// Verify the status code is 200.
		if err := VerifyStatusCode(response.Response, http.StatusOK); err != nil {
			return "", err
		}

		// Find the latest release tag for the given network
		for _, release := range releases {
			if strings.HasPrefix(*release.TagName, prefix) {
				num, err := strconv.Atoi(strings.TrimPrefix(*release.TagName, prefix))
				if err != nil {
					continue
				}
				if num > max {
					max = num
					tag = *release.TagName
				}
			}
		}

		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	if tag == "" {
		return "", fmt.Errorf("cannot find any release for %v", network)
	}
	return tag, nil
}
