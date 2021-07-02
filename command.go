package nodectl

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fatih/color"
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

// listAllNodes will display detail information of your darknodes. Tags can be
// provided to only show darknodes have the tags
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
		go func(i int) {
			wg.Add(1)
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

	fmt.Printf("%-20s | %-45s | %-30s | %-15s | %-8s | %-15s | %-15s\n", "name", "ethereum address", "id", "ip", "provider", "tags", "version")
	for _, info := range infos {
		if info.Name != "" {
			fmt.Printf("%v", info.String())
		}
	}

	// Print error of nodes which we cannot get the info
	if errNum > 0 {
		for i, err := range errs {
			if err != nil {
				color.Red("%v %v", nodesNames[i], err.Error())
			}
		}
	}
	return nil
}

type NodeInfo struct {
	Name     string
	ID       string
	IP       string
	EthAddr  string
	Provider string
	Tags     string
	Version  string
}

func (info NodeInfo) String() string {
	return fmt.Sprintf("%-20s | %-30s | %-45s | %-15s | %-8s | %-15s | %-15s",
		info.Name,
		info.ID,
		info.IP,
		info.Provider,
		info.Tags,
		info.EthAddr,
		info.Version,
	)
}

func GetNodeInfo(name string) (NodeInfo, error) {
	if err := util.ValidateNodeName(name); err != nil {
		return NodeInfo{}, nil
	}

	// TODO : GET THE NODE ID, ETH ADDRESS AND VERSION
	id := ""
	ethAddr := ""
	version := "0.0.0"

	ip, err := util.NodeIP(name)
	if err != nil {
		return NodeInfo{}, nil
	}
	provider, err := util.NodeProvider(name)
	if err != nil {
		return NodeInfo{}, nil
	}
	tagFile := filepath.Join(util.NodePath(name), "tags.out")
	tagsBytes, err := ioutil.ReadFile(tagFile)
	if err != nil {
		return NodeInfo{}, nil
	}
	tags := strings.TrimSpace(string(tagsBytes))

	return NodeInfo{
		Name:     name,
		ID:       id,
		IP:       ip,
		Provider: provider,
		Tags:     tags,
		EthAddr:  ethAddr,
		Version:  version,
	}, nil
}
