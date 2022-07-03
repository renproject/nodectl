package nodectl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/google/go-github/v44/github"
	"github.com/renproject/aw/wire"
	"github.com/renproject/nodectl/provider"
	"github.com/renproject/nodectl/renvm"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
)

// Commands for different actions to darknodes.
var (
	ActionStart   = "systemctl --user start darknode"
	ActionStop    = "systemctl --user stop darknode"
	ActionRestart = "systemctl --user restart darknode"
)

// updateServiceStatus can update status of the darknode service.
func updateServiceStatus(ctx *cli.Context, cmd string) error {
	tags := ctx.String("tags")
	name := ctx.Args().First()

	// Get the script we want to run depends on the command.
	var script, message string
	switch cmd {
	case "start":
		script, message = ActionStart, "started"
	case "stop":
		script, message = ActionStop, "stopped"
	case "restart":
		script, message = ActionRestart, "restarted"
	default:
		panic(fmt.Sprintf("invalid switch command = %v", cmd))
	}

	// Parse the names of the darknode we want to operate
	nodes, err := util.ParseNodesFromNameAndTags(name, tags)
	if err != nil {
		return err
	}
	errs := make([]error, len(nodes))
	wg := new(sync.WaitGroup)
	for i := range nodes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			username := util.NodeInstanceUser(nodes[i])
			errs[i] = util.RemoteRun(nodes[i], script, username)
			if errs[i] == nil {
				color.Green("[%v] has been %v.", nodes[i], message)
			} else {
				color.Red("failed to %v [%v]: %v", script, nodes[i], errs[i])
			}
		}(i)
	}
	wg.Wait()
	return util.HandleErrs(errs)
}

// listAllNodes will display detail information of your Darknodes. Tags can be
// provided to only show Darknodes have the tags
func listAllNodes(ctx *cli.Context) error {
	tags := ctx.String("tags")
	nodesNames, err := util.GetNodesByTags(tags)
	if err != nil {
		return err
	}

	// Fetch darknodes details in parallel
	wg := new(sync.WaitGroup)
	infos := make([]NodeInfo, len(nodesNames))
	errs := make([]error, len(nodesNames))
	var errNum int64
	for i := range nodesNames {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			info, err := GetNodeInfo(nodesNames[i])
			if err != nil {
				errs[i] = err
				atomic.AddInt64(&errNum, 1)
				return
			}
			infos[i] = info
		}(i)
	}
	wg.Wait()

	// Display the darknodes info in a formatted table.
	if errNum == int64(len(nodesNames)) {
		color.Red("Fail to fetch nodes info.")
		errMessages := make([]string, 0, len(errs))
		for i, err := range errs {
			if err != nil {
				errMessages = append(errMessages, fmt.Sprintf("%v %v", nodesNames[i], err.Error()))
			}
		}
		color.Red(strings.Join(errMessages, "\n"))
	}

	fmt.Printf("%-20s | %-45s | %-15s | %-8s | %-15s\n", "name", "ethereum address", "ip", "provider", "tags")
	for _, info := range infos {
		if info.Name != "" {
			fmt.Printf("%v", info.String())
		}
	}

	// // Print error of nodes which we cannot get the info
	// // TODO : Might be good to print error messages when having a `-v` or `-debug` flag
	// if errNum > 0 {
	// 	for i, err := range errs {
	// 		if err != nil {
	// 			color.Red("%v %v", nodesNames[i], err.Error())
	// 		}
	// 	}
	// }
	return nil
}

type NodeInfo struct {
	Name     string
	IP       string
	EthAddr  string
	Provider string
	Tags     string
}

func (info NodeInfo) String() string {
	return fmt.Sprintf("%-20s | %-45s | %-15s | %-8s | %-15s\n",
		info.Name,
		info.EthAddr,
		info.IP,
		info.Provider,
		info.Tags,
	)
}

func GetNodeInfo(name string) (NodeInfo, error) {
	if err := util.NodeExistence(name); err != nil {
		return NodeInfo{}, err
	}

	config, err := util.NodeOptions(name)
	if err != nil {
		return NodeInfo{}, err
	}
	ethAddr := util.NodeEthereumAddr(config.PrivKey)
	ip, err := util.NodeIP(name)
	if err != nil {
		return NodeInfo{}, err
	}
	provider, err := util.NodeProvider(name)
	if err != nil {
		return NodeInfo{}, err
	}
	tagFile := filepath.Join(util.NodePath(name), "tags.out")
	tagsBytes, err := ioutil.ReadFile(tagFile)
	if err != nil {
		return NodeInfo{}, err
	}
	tags := strings.TrimSpace(string(tagsBytes))

	return NodeInfo{
		Name:     name,
		IP:       ip,
		Provider: provider,
		Tags:     tags,
		EthAddr:  ethAddr.Hex(),
	}, nil
}

func UpdateDarknode(ctx *cli.Context) error {
	name := ctx.Args().First()
	tags := ctx.String("tags")
	dep := ctx.Bool("dep")
	config := ctx.Bool("config")
	version := strings.TrimSpace(ctx.String("version"))

	// Parse nodes from the name/tags
	nodes, err := util.ParseNodesFromNameAndTags(name, tags)
	if err != nil {
		return err
	}
	options, err := util.NodeOptions(nodes[0])
	if err != nil {
		return err
	}
	network := options.Network

	// Use latest version if user doesn't provide a version number
	if version != "" {
		if err := validateVersion(version); err != nil {
			return err
		}
	} else {
		version, err = util.LatestRelease(network)
		if err != nil {
			return err
		}
	}

	// Get the config template if we need to update the config
	var newOptions renvm.Options
	if config {
		optionsURL := util.OptionsURL(options.Network)
		newOptions, err = renvm.OptionTemplate(optionsURL)
		if err != nil {
			return fmt.Errorf("fetching latest options template: %v", err)
		}
	}

	// Updating darknodes
	color.Green("Updating darknodes...")
	errs := make([]error, len(nodes))
	wg := new(sync.WaitGroup)
	for i := range nodes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			errs[i] = update(nodes[i], version, dep, newOptions)
			if errs[i] == nil {
				color.Green("- ✅ [%v] has been updated.", nodes[i])
			}
		}(i)
	}
	wg.Wait()

	return util.HandleErrs(errs)
}

func RecoverDarknode(ctx *cli.Context) error {
	name := ctx.Args().First()
	tags := ctx.String("tags")
	force := ctx.Bool("force")
	snapshot := ctx.String("snapshot")

	// Confirmation prompt if force prompt is not present
	if !force {
		color.Yellow("This will clear your darknode database and reset everything to the latest snapshot")
		color.Yellow("Are you sure you want to recover? (y/N)")
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		input := strings.ToLower(strings.TrimSpace(text))
		if input != "yes" && input != "y" {
			return nil
		}
	}

	// Validate all the input parameters
	nodes, err := util.ParseNodesFromNameAndTags(name, tags)
	if err != nil {
		return err
	}

	wg := new(sync.WaitGroup)
	for i := range nodes {
		wg.Add(1)
		node := nodes[i]

		go func() {
			defer wg.Done()

			options, err := util.NodeOptions(node)
			if err != nil {
				color.Red("cannot read darknode %v config file, err = %v", node, err)
				return
			}

			// Stop the darknode service
			color.Green("[%v] stop darknode", node)
			stopService := "systemctl --user stop darknode"
			if err := util.RemoteRun(name, stopService, "darknode"); err != nil {
				color.Red("failed to stop darknode service, err = %v", err)
				return
			}

			// Download the snapshot and replace the current database
			snapshotURL := util.SnapshotURL(options.Network, snapshot)
			script := fmt.Sprintf("cd .darknode && rm -rf db chain.wal genesis.json && curl -sSOJL %v && tar xzf latest.tar.gz && rm latest.tar.gz", snapshotURL)
			if err := util.RemoteRun(name, script, "darknode"); err != nil {
				color.Red("failed to fetch snapshot file, err = %v", err)
				return
			}

			// Restart the darknode
			restartService := "systemctl --user restart darknode"
			if err := util.RemoteRun(name, restartService, "darknode"); err != nil {
				color.Red("failed to restart darknode service, err = %v", err)
				return
			}
			color.Green("[%v] is recovered", node)
		}()
	}
	wg.Wait()
	return nil
}

func update(name, ver string, dep bool, template renvm.Options) error {
	// Update the dependency for darknode if needed
	if dep {
		color.Green("- Updating [%v] dependency", name)
		if err := updateDependency(name); err != nil {
			return err
		}
	}

	color.Green("- Updating [%v] to version %v", name, ver)

	// Fetch the latest config template and update the darknode's config
	configScript := ""
	if len(template.Peers) > 0 {
		newOptions, err := updateConfig(name, template)
		if err != nil {
			return err
		}
		newOptionsAsBytes, err := json.MarshalIndent(newOptions, "", " ")
		if err != nil {
			return err
		}
		configScript = fmt.Sprintf(`&& echo '%v' > ~/.darknode/config.json`, string(newOptionsAsBytes))
	}

	// Update binary and config in the remote instance
	username := util.NodeInstanceUser(name)
	url := fmt.Sprintf("https://www.github.com/renproject/darknode-release/releases/download/%v", ver)
	script := fmt.Sprintf(`curl -sL %v/darknode > ~/.darknode/bin/darknode-new && 
mv ~/.darknode/bin/darknode-new ~/.darknode/bin/darknode &&
chmod +x ~/.darknode/bin/darknode %v && systemctl --user restart darknode`, url, configScript)

	return util.RemoteRun(name, script, username)
}

func validateVersion(version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := github.NewClient(nil)
	_, response, err := client.Repositories.GetReleaseByTag(ctx, "renproject", "darknode-release", version)
	if err != nil {
		return err
	}

	// Check the status code of the response
	switch response.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("cannot find release [%v] on github", version)
	default:
		data, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("cannot connect to github, code = %v, err = %v", response.StatusCode, string(data))
	}
}

func updateDependency(name string) error {
	color.Green("- Updating [%v] dependency", name)
	username, err := provider.NodeSudoUsername(name)
	if err != nil {
		return err
	}
	script := `wget -q https://github.com/CosmWasm/wasmvm/archive/v0.16.1.tar.gz &&
tar -xzf v0.16.1.tar.gz && 
sudo cp ./wasmvm-0.16.1/api/libwasmvm.so /usr/lib/ && 
rm -r v0.16.1.tar.gz wasmvm-0.16.1`
	return util.RemoteRun(name, script, username)
}

func updateConfig(name string, template renvm.Options) (renvm.Options, error) {
	options, err := util.NodeOptions(name)
	if err != nil {
		return renvm.Options{}, fmt.Errorf("reading config file: %v", err)
	}
	newOptions := template
	newOptions.PrivKey = options.PrivKey
	newOptions.Home = options.Home

	// Check the config template has our address
	_, index, err := util.FindSelfAddress(newOptions)
	if err != nil {
		return renvm.Options{}, err
	}
	// Add our address to the peer list if not found from the config template
	if index == -1 {
		self, _, err := util.FindSelfAddress(options)
		if err != nil {
			return renvm.Options{}, err
		}
		newOptions.Peers = append([]wire.Address{self}, newOptions.Peers...)
	}

	// Update our local version of the config file
	path := filepath.Join(util.NodePath(name), "config.json")
	if err := renvm.OptionsToFile(newOptions, path); err != nil {
		return renvm.Options{}, fmt.Errorf("update local config : %v", err)
	}
	return newOptions, nil
}
