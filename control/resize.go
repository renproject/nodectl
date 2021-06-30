package darknode

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"github.com/fatih/color"
	"github.com/renproject/darknode-cli/darknode/provider"
	"github.com/renproject/darknode-cli/util"
	"github.com/urfave/cli/v2"
)

// ErrInvalidInstanceSize is returned when the given instance size is invalid.
var (
	ErrInvalidInstanceSize = errors.New("invalid instance size")
)

// Regex for all the providers used for updating terraform config files.
var (
	RegexAws = `instance_type\s+=\s*"(?P<instance>.+)"`

	RegexDo = `size\s+=\s*"(?P<instance>.+)"`

	RegexGcp = `machine_type\s+=\s+"(?P<instance>.+)"`
)

func resize(ctx *cli.Context) error {
	name := ctx.Args().Get(0)
	if err := util.ValidateNodeName(name); err != nil {
		return err
	}
	newSize := ctx.Args().Get(1)
	if newSize == "" {
		return ErrInvalidInstanceSize
	}

	p, err := provider.GetProvider(name)
	if err != nil {
		return err
	}

	switch p {
	case provider.NameAws:
		replacement := fmt.Sprintf(`instance_type   = "%v"`, newSize)
		return applyChanges(name, RegexAws, replacement)
	case provider.NameDo:
		replacement := fmt.Sprintf(`size        = "%v"`, newSize)
		return applyChanges(name, RegexDo, replacement)
	case provider.NameGcp:
		replacement := fmt.Sprintf(`machine_type = "%v"`, newSize)
		return applyChanges(name, RegexGcp, replacement)
	default:
		panic("unknown provider")
	}
}

func applyChanges(name, regex, replacement string) error {
	reg, err := regexp.Compile(regex)
	if err != nil {
		return err
	}

	// Update the main.tf file.
	path := filepath.Join(util.NodePath(name), "main.tf")
	tf, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	newTf := reg.ReplaceAll(tf, []byte(replacement))
	if err := ioutil.WriteFile(path, newTf, 0600); err != nil {
		return err
	}

	// Apply the changes using terraform
	color.Green("Resizing dark nodes ...")
	apply := fmt.Sprintf("cd %v && terraform apply -auto-approve -no-color", util.NodePath(name))
	if err := util.Run("bash", "-c", apply); err != nil {
		// revert the `main.tf` file if fail to resize the droplet
		if err := ioutil.WriteFile(path, tf, 0600); err != nil {
			fmt.Println("fail to revert the change to `main.tf` file")
		}
		color.Red("Invalid instance type, please try again with a valid one.")
	}
	return nil
}
