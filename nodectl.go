package nodectl

import (
	"path/filepath"

	"github.com/fatih/color"
	"github.com/renproject/nodectl/provider"
	"github.com/renproject/nodectl/util"
	"github.com/urfave/cli/v2"
)

// App creates a new cli application.
func App() *cli.App {
	app := cli.NewApp()
	app.Name = "nodectl"
	app.Usage = "A command-line tool for managing RenVM nodes."
	app.EnableBashCompletion = true

	// Define sub-commands
	app.Commands = []*cli.Command{
		{
			Name:  "up",
			Usage: "Deploy a new Ren node",
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
			Usage:   "Destroy one of your Ren node",
			Aliases: []string{"down"},
			Flags:   []cli.Flag{TagsFlag, ForceFlag},
			Action: func(c *cli.Context) error {
				// TODO : FINISH THIS
				panic("unimplemented")
			},
		},
		// {
		// 	Name:  "update",
		// 	Usage: "Update your Ren nodes to the latest version",
		// 	Flags: []cli.Flag{TagsFlag, VersionFlag, DowngradeFlag},
		// 	Action: func(c *cli.Context) error {
		// 		return updateNode(c)
		// 	},
		// },
		{
			Name:  "ssh",
			Flags: []cli.Flag{},
			Usage: "SSH into one of your Ren node",
			Action: func(c *cli.Context) error {
				name := c.Args().First()
				if err := util.ValidateNodeName(name); err != nil {
					return err
				}
				ip, err := util.NodeIP(name)
				if err != nil {
					return err
				}
				keyPath := filepath.Join(util.NodePath(name), "ssh_keypair")
				return util.Run("ssh", "-i", keyPath, "node@"+ip, "-oStrictHostKeyChecking=no")
			},
		},
		{
			Name:  "start",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Start a single node or a set of nodes by its tag",
			Action: func(c *cli.Context) error {
				return updateServiceStatus(c, "start")
			},
		},
		{
			Name:  "stop",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Stop a single node or a set of nodes by its tag",
			Action: func(c *cli.Context) error {
				return updateServiceStatus(c, "stop")
			},
		},
		{
			Name:  "restart",
			Flags: []cli.Flag{TagsFlag},
			Usage: "Restart a single node or a set of nodes by its tag",
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
	}

	// Show error message and display the help page when command is not found.
	app.CommandNotFound = func(c *cli.Context, command string) {
		color.Red("[Warning] command '%q' not found", command)
		color.Red("[Warning] run 'nodectl --help' for a list of available commands")
	}

	return app
}
