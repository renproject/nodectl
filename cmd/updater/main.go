package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/joho/godotenv"
	"github.com/renproject/nodectl/renvm"
	"github.com/renproject/nodectl/util"
)

const (
	DefaultBinInterval     = 12 * time.Hour
	DefaultConfigInterval  = time.Minute
	DefaultRecoverInterval = time.Minute

	KeyInstalledVersion  = "DARKNODE_INSTALLED"
	KeyConfigVersionID   = "DARKNODE_CONFIG_VERSIONID"
	KeySnapshotVersionID = "DARKNODE_SNAPSHOT_VERSIONID"

	EnvUpdateBIN      = "UPDATE_BIN"
	EnvUpdateConfig   = "UPDATE_CONFIG"
	EnvUpdateRecovery = "UPDATE_RECOVERY"
)

// An auto-updater to help the Darknode keep updates in the network
func main() {
	sigsChan := make(chan os.Signal, 1)
	signal.Notify(sigsChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// Create a global context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the network of the darknode
	store := NewEnvStore()
	path := filepath.Join(os.Getenv("HOME"), ".darknode", "config.json")
	options, err := renvm.NewOptionsFromFile(path)
	if err != nil {
		log.Printf("unable to fetch darknode config, err = %v", err)
		return
	}
	network := options.Network

	// Initialise the s3 service
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-southeast-1"),
	})
	if err != nil {
		log.Printf("unable to initialise the s3 service, err = %v", err)
		return
	}
	service := s3.New(sess)

	// Check and update darknode binary periodically
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Skip if binary update has been disabled
				if store.Get(EnvUpdateBIN) != "1" {
					log.Printf("skip binanry update since disabled")
					break
				}

				// Fetch the latest release version
				latestVer, err := util.LatestRelease(network)
				if err != nil {
					log.Printf("unable to fetch latest release version, err = %v", err)
					break
				}

				// Get the installed version
				installedVer := store.Get(KeyInstalledVersion)
				if installedVer == "" {
					log.Printf("unable to get installed version")
					break
				}

				// Compare two versions
				log.Printf("latestes version = %v, installed = %v", latestVer, installedVer)
				res, err := VersionCompare(latestVer, installedVer)
				if err != nil {
					log.Printf("invalid version number, err = %v", err)
					break
				}
				if res != 1 {
					break
				}

				// Update the binary if needed
				log.Printf("updating the binary...")
				updateScript := fmt.Sprintf("curl -sL https://github.com/renproject/darknode-release/releases/download/%v/darknode > ~/.darknode/bin/darknode", latestVer)
				if err := util.Run("bash", "-c", updateScript); err != nil {
					log.Printf("unable to download darknode binary, err = %v", err)
					break
				}

				// Restart the service
				RestartDarknodeService()
			}

			interval, err := time.ParseDuration(os.Getenv("BIN_INTERVAL"))
			if err != nil {
				interval = time.Minute
			}
			time.Sleep(interval)
		}
	}()

	// Check and update config file updates
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Skip if config update has been disabled
				if store.Get(EnvUpdateConfig) != "1" {
					log.Printf("skip config update since disabled")
					break
				}

				// Get the config version we have
				installedVerID := store.Get(KeyConfigVersionID)
				if installedVerID == "" {
					log.Printf("unable to get darknode version ID")
					break
				}

				// Fetch the latest config version
				input := &s3.GetObjectInput{
					Bucket: aws.String("darknode.renproject.io"),
					Key:    aws.String(fmt.Sprintf("%v/config.json", network)),
				}
				obj, err := service.GetObject(input)
				if err != nil {
					log.Printf("unable to get config object from s3, err = %v", err)
					break
				}
				latestVerID := obj.VersionId

				log.Printf("latest config version = %v, instaleld config version = %v", latestVerID, installedVerID)
				// Update the config if needed
				if &installedVerID == latestVerID {
					break
				}
				log.Printf("updating the config...")
				latestOptions, err := renvm.NewOptionsFromFile(util.OptionsURL(network))
				if err != nil {
					log.Printf("unable to fetch latest options from s3, err = %v", err)
					break
				}
				options.Chains = latestOptions.Chains
				options.Selectors = latestOptions.Selectors
				data, err := json.MarshalIndent(options, "", "    ")
				if err != nil {
					break
				}
				copyConfig := fmt.Sprintf("echo '%s' > $HOME/.darknode/config.json", string(data))
				if err := util.Run("bash", "-c", copyConfig); err != nil {
					break
				}

				// Restart the service
				RestartDarknodeService()
			}

			interval, err := time.ParseDuration(os.Getenv("CONFIG_INTERVAL"))
			if err != nil {
				interval = DefaultConfigInterval
			}
			time.Sleep(interval)
		}
	}()

	// Watch for snapshots for recovery
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Skip if auto recovery has been disabled
				if store.Get(EnvUpdateRecovery) != "1" {
					log.Printf("skip config update since disabled")
					break
				}

				// Get the config version we have
				installedVerID := store.Get(KeySnapshotVersionID)
				if installedVerID == "" {
					log.Printf("unable to get snapshot version ID")
					break
				}

				// Fetch the latest snapshot version
				input := &s3.GetObjectInput{
					Bucket: aws.String("darknode.renproject.io"),
					Key:    aws.String(fmt.Sprintf("%v/latest.tar.gz", network)),
				}
				obj, err := service.GetObject(input)
				if err != nil {
					log.Printf("unable to get snapshot object from s3, err = %v", err)
					break
				}
				latestVerID := obj.VersionId

				log.Printf("latest config version = %v, instaleld config version = %v", latestVerID, installedVerID)
				if *latestVerID == installedVerID {
					break
				}

				log.Printf("doing an recovery...")
				snapshotURL := util.SnapshotURL(options.Network, "")
				script := fmt.Sprintf("cd $HOME/.darknode && rm -rf chain.wal genesis.json && mv db db-bak && curl -sSOJL %v && tar xzf latest.tar.gz && rm latest.tar.gz", snapshotURL)
				if err := util.Run("bash", "-c", script); err != nil {
					color.Red("failed to fetch snapshot file, err = %v", err)
					return
				}

				// Restart the service
				RestartDarknodeService()
			}

			interval, err := time.ParseDuration(os.Getenv("RECOVERY_INTERVAL"))
			if err != nil {
				interval = DefaultRecoverInterval
			}
			time.Sleep(interval)
		}
	}()

	<-sigsChan
}

type EnvStore struct {
	mu   *sync.Mutex
	path string
}

func NewEnvStore() *EnvStore {
	path := filepath.Join(os.Getenv("HOME"), ".darknode", ".env")

	return &EnvStore{
		mu:   new(sync.Mutex),
		path: path,
	}
}

func (store *EnvStore) Get(name string) string {
	store.mu.Lock()
	defer store.mu.Unlock()

	envs, err := godotenv.Read(store.path)
	if err != nil {
		return ""
	}
	return envs[name]
}

func (store *EnvStore) Set(key, value string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	envs, err := godotenv.Read(store.path)
	if err != nil {
		return err
	}
	envs[key] = value
	return godotenv.Write(envs, store.path)
}

func VersionCompare(ver1Str, ver2Str string) (int, error) {
	ver1, err := version.NewVersion(ver1Str)
	if err != nil {
		return 0, err
	}

	ver2, err := version.NewVersion(ver2Str)
	if err != nil {
		return 0, err
	}
	return ver1.Compare(ver2), nil
}

func RestartDarknodeService() {
	log.Printf("restarting darknode service")
	script := "systemctl --user restart darknode"
	if err := util.Run("bash", "-c", script); err != nil {
		log.Printf("unable to restart darknode service, err = %v", err)
	}
}
