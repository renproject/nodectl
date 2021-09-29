package nodectl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/renproject/nodectl/provider"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
)

// App creates a new cli application.
func App() *cli.App {
	app := cli.NewApp()
	app.Name = "nodectl"
	app.Usage = "A command-line tool for managing Darknodes."
	app.EnableBashCompletion = true

	// Define sub-commands
	app.Commands = []*cli.Command{
		{
			Name:  "up",
			Usage: "Deploy a new Darknode",
			Flags: []cli.Flag{
				// General
				NameFlag, TagsFlag, NetworkFlag, ConfigFlag,
				// AWS
				AwsFlag, AwsAccessKeyFlag, AwsSecretKeyFlag, AwsInstanceFlag, AwsRegionFlag, AwsProfileFlag,
				// Digital Ocean
				DoFlag, DoRegionFlag, DoSizeFlag, DoTokenFlag,
				// Google Cloud Platform
				GcpFlag, GcpRegionFlag, GcpCredFlag, GcpMachineFlag,
			},
			Action: func(ctx *cli.Context) error {
				// Parse the provider and deploy the node
				p, err := provider.ParseProvider(ctx)
				if err != nil {
					return err
				}
				return p.Deploy(ctx)
			},
		},
		{
			Name:    "destroy",
			Usage:   "Destroy one of your Darknode",
			Aliases: []string{"down"},
			Flags:   []cli.Flag{TagsFlag, ForceFlag},
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				force := c.Bool("force")
				path := util.NodePath(name)

				if err := util.CheckNodeExistence(name); err != nil {
					return err
				}

				// Confirmation prompt if force prompt is not present
				if !force {
					fmt.Println("Are you sure you want to destroy your Darknode? (y/N)")
					reader := bufio.NewReader(os.Stdin)
					text, _ := reader.ReadString('\n')
					input := strings.ToLower(strings.TrimSpace(text))
					if input != "yes" && input != "y" {
						return nil
					}
				}

				color.Green("Backing up config...")
				if err := util.BackUpConfig(name); err != nil {
					return err
				}

				color.Green("Destroying your Darknode...")
				destroy := fmt.Sprintf("cd %v && terraform destroy --auto-approve && cd .. && rm -rf %v", path, name)
				return util.Run("bash", "-c", destroy)
			},
		},
		{
			Name:  "update",
			Usage: "Update your Darknode to the latest version",
			Flags: []cli.Flag{TagsFlag, VersionFlag},
			Action: func(c *cli.Context) error {
				return UpdateDarknode(c)
			},
		},
		{
			Name:  "recover",
			Usage: "Recover you Darknode from broken state",
			Flags: []cli.Flag{TagsFlag, GenesisFlag},
			Action: func(c *cli.Context) error {
				return RecoverDarknode(c)
			},
		},
		{
			Name:  "ssh",
			Flags: []cli.Flag{},
			Usage: "SSH into one of your Darknode",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if err := util.CheckNodeExistence(name); err != nil {
					return err
				}
				ip, err := util.NodeIP(name)
				if err != nil {
					return err
				}
				keyPath := filepath.Join(util.NodePath(name), "ssh_keypair")
				return util.Run("ssh", "-i", keyPath, "darknode@"+ip, "-oStrictHostKeyChecking=no")
			},
		},
		{
			Name:  "start",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Start a single Darknode or a set of Darknodes by its tag",
			Action: func(c *cli.Context) error {
				return updateServiceStatus(c, "start")
			},
		},
		{
			Name:  "stop",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Stop a single Darknode or a set of Darknodes by its tag",
			Action: func(c *cli.Context) error {
				return updateServiceStatus(c, "stop")
			},
		},
		{
			Name:  "restart",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Restart a single Darknode or a set of Darknodes by its tag",
			Action: func(c *cli.Context) error {
				return updateServiceStatus(c, "restart")
			},
		},
		{
			Name:  "list",
			Usage: "List information about all of your Darknodes",
			Flags: []cli.Flag{TagsFlag},
			Action: func(c *cli.Context) error {
				return listAllNodes(c)
			},
		},
		{
			Name:  "address",
			Usage: "Show the signed address of the node",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if err := util.CheckNodeExistence(name); err != nil {
					return err
				}
				ip, err := util.NodeIP(name)
				if err != nil {
					return err
				}
				opts, err := util.NodeOptions(name)
				if err != nil {
					return err
				}
				for _, peer := range opts.Peers {
					if strings.HasPrefix(peer.Value, ip) {
						data, err := json.MarshalIndent(peer, "", "    ")
						if err != nil {
							return err
						}
						color.Green("%s", data)
						return nil
					}
				}
				return fmt.Errorf("cannot fetch darknode address")
			},
		},
	}

	// Show error message and display the help page when command is not found.
	app.CommandNotFound = func(c *cli.Context, command string) {
		color.Red("[Warning] command '%q' not found", command)
		color.Red("[Warning] run 'nodectl --help' for a list of available commands")
	}

	return app
}
