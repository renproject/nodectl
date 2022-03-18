package util

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
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

func LatestRelease(network multichain.Network) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reg := regexp.MustCompile("^(\\d+.\\d+(.\\d+){0,1})-(mainnet|testnet|devnet)(\\d+)$")
	maxIndex := 0
	maxVersion, _ := version.NewVersion("0.0.0")
	tag := ""

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
			// Parse version, network, index from the release tag name
			// i.e. "0.4.10-mainnet12"  -> ["0.4.10-mainnet12", "0.4.10", ".10", "mainnet", "12"]
			matches := reg.FindStringSubmatch(*release.TagName)
			if len(matches) != reg.NumSubexp()+1 {
				continue
			}
			releaseVersion, err := version.NewVersion(matches[1])
			if err != nil {
				continue
			}
			releaseNetwork := multichain.Network(matches[3])
			releaseIndex, err := strconv.Atoi(matches[4])
			if err != nil {
				continue
			}

			// Continue if it's not a different network
			if releaseNetwork != network {
				continue
			}

			// Compare the version and index
			if releaseVersion.GreaterThan(maxVersion) {
				maxVersion = releaseVersion
				maxIndex = releaseIndex
				tag = *release.TagName
			} else if releaseVersion.Equal(maxVersion) {
				if releaseIndex > maxIndex {
					maxIndex = releaseIndex
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
