package darknode

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync/atomic"

	"github.com/fatih/color"
	"github.com/renproject/darknode-cli/darknode/provider"
	"github.com/renproject/darknode-cli/util"
	"github.com/renproject/phi"
	"github.com/urfave/cli/v2"
)

// listAllNodes will list information of deployed darknodes. Results can be filtered by the tags.
func listAllNodes(ctx *cli.Context) error {
	tags := ctx.String("tags")
	nodesNames, err := util.GetNodesByTags(tags)
	if err != nil {
		return err
	}

	nodes := make([][]string, len(nodesNames))
	var errs int64
	phi.ParForAll(nodesNames, func(i int) {
		name := nodesNames[i]
		var err error
		nodes[i], err = func() ([]string, error) {
			id, err := util.ID(name)
			if err != nil {
				return nil, err
			}
			ip, err := util.IP(name)
			if err != nil {
				return nil, err
			}
			provider, err := provider.GetProvider(name)
			if err != nil {
				return nil, err
			}
			tagFile := filepath.Join(util.NodePath(nodesNames[i]), "tags.out")
			tags, err := ioutil.ReadFile(tagFile)
			if err != nil {
				return nil, err
			}
			ethAddr, err := id.ToEthereumAddress()
			if err != nil {
				return nil, err
			}
			version := util.Version(name)
			return []string{name, id.String(), ip, provider, string(tags), ethAddr.Hex(), version}, nil
		}()
		if err != nil {
			color.Red("[%v] cannot get detail of the darknode, err = %v", name, err)
			atomic.AddInt64(&errs, 1)
		}
	})

	// Check if we can find any valid nodes.
	if atomic.LoadInt64(&errs) == int64(len(nodesNames)) {
		return fmt.Errorf("cannot find any node")
	}

	fmt.Printf("%-20s | %-30s | %-15s | %-8s | %-15s | %-45s | %-15s\n", "name", "id", "ip", "provider", "tags", "ethereum address", "version")
	for _, node := range nodes {
		if node != nil {
			fmt.Printf("%-20s | %-30s | %-15s | %-8s | %-15s | %-45s | %-15s\n", node[0], node[1], node[2], node[3], node[4], node[5], node[6])
		}
	}
	return nil
}
