package nodectl

import (
	"context"
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
	"github.com/google/go-github/v36/github"
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
			errs[i] = util.RemoteRun(nodes[i], script, "darknode")
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
	if err := util.CheckNodeExistence(name); err != nil {
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
	version := strings.TrimSpace(ctx.String("version"))
	nodes, err := util.ParseNodesFromNameAndTags(name, tags)
	if err != nil {
		return err
	}

	// Use latest version if user doesn't provide a version number
	color.Green("Verifying darknode release ...")
	if version == "" {
		version, err = util.LatestStableRelease()
		if err != nil {
			return err
		}
	} else {
		if err := validateVersion(version); err != nil {
			return err
		}
	}

	// Updating darknodes
	color.Green("Updating darknodes to %v...", version)
	errs := make([]error, len(nodes))
	wg := new(sync.WaitGroup)
	for i := range nodes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			errs[i] = update(nodes[i], version)
		}(i)
	}
	wg.Wait()

	return util.HandleErrs(errs)
}

func RecoverDarknode(ctx *cli.Context) error {
	name := ctx.Args().First()
	tags := ctx.String("tags")
	dbPath := ctx.String("db")
	genesisPath := ctx.String("genesis")

	// Validate all the input parameters
	nodes, err := util.ParseNodesFromNameAndTags(name, tags)
	if err != nil {
		return err
	}
	dbPath, err = filepath.Abs(dbPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("no such file %v", dbPath)
	}
	if !strings.HasSuffix(dbPath, "tar.gz") {
		return fmt.Errorf("invalid database format")
	}
	_, err = renvm.NewGenesisFromFile(genesisPath)
	if err != nil {
		return err
	}

	// Update each nodes
	for _, name := range nodes {
		// Upload the genesis file
		data, err := ioutil.ReadFile(genesisPath)
		if err != nil {
			return err
		}
		genesisScript := fmt.Sprintf("cp ~/.darknode/genesis.json ~/.darknode/genesis-bak.json && echo '%s' > $HOME/.darknode/genesis.json", string(data))
		if err := util.RemoteRun(name, genesisScript, "darknode"); err != nil {
			return err
		}

		// Upload the database file
		if err := util.SCP(name, genesisPath, "/home/darknode/.darknode/database.tar.gz"); err != nil {
			return err
		}
		dbScript := "mv ~/.darknode/db ~/.darknode/db-bak && tar xzvf database.tar.gz && rm database.tar.gz4"
		if err := util.RemoteRun(name, dbScript, "darknode"); err != nil {
			return err
		}

		// Restart the darknode
		restartService := "systemctl --user restart darknode"
		if err := util.RemoteRun(name, restartService, "darknode"); err != nil {
			return err
		}
		color.Green("[%v] has been recovered", name)
	}
	return nil
}

func update(name, ver string) error {
	url := fmt.Sprintf("https://www.github.com/renproject/darknode-release/releases/download/%v", ver)
	script := fmt.Sprintf(`curl -sL %v/darknode > ~/.darknode/bin/darknode-new && 
mv ~/.darknode/bin/darknode-new ~/.darknode/bin/darknode &&
chmod +x ~/.darknode/bin/darknode && systemctl --user restart darknode`, url)
	return util.RemoteRun(name, script, "darknode")
}

func validateVersion(version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		return fmt.Errorf("cannot connect to github, code= %v, err = %v", response.StatusCode, string(data))
	}
}
