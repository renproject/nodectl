package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/id"
	"github.com/renproject/nodectl/renvm"
)

var (
	// ErrEmptyNameAndTags is returned when both name and tags are not given.
	ErrEmptyNameAndTags = errors.New("please provide name or tags of the node you want to operate")

	// ErrTooManyArguments is returned when both name and tags are given.
	ErrTooManyArguments = errors.New("too many arguments, cannot have both name and tags")

	// ErrEmptyName is returned when user gives an empty node name.
	ErrEmptyName = errors.New("node name cannot be empty")
)

// ParseNodesFromNameAndTags returns the darknode names which satisfies the name
// requirements or the tag requirements.
func ParseNodesFromNameAndTags(name, tags string) ([]string, error) {
	if name == "" && tags == "" {
		return nil, ErrEmptyNameAndTags
	} else if name == "" && tags != "" {
		return GetNodesByTags(tags)
	} else if name != "" && tags == "" {
		return []string{name}, CheckNodeExistence(name)
	} else {
		return nil, ErrTooManyArguments
	}
}

// ValidateName validates the given name. It should
// 1) Only contains letter, number, "-" and "_".
// 2) Not more than 32 characters
// 3) Not start or end with a whitespace.
func ValidateName(name string) error {
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("name cannot have whitespace at beginning or end")
	}

	nameRegex, err := regexp.Compile("^[a-zA-Z0-9_-]{1,32}$")
	if err != nil {
		return err
	}
	if !nameRegex.MatchString(name){
		return fmt.Errorf("no special character and total length should be less than 32 characters")
	}
	return nil
}

// CheckNodeExistence checks if there exists a node with given name.
func CheckNodeExistence(name string) error {
	files, err := ioutil.ReadDir(filepath.Join(Directory, "/darknodes"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.Name() == name {
			return nil
		}
	}
	return fmt.Errorf("darknode [%v] not found", name)
}

// NodeOptions returns the config of the node with given name.
func NodeOptions(name string) (renvm.Options, error) {
	path := filepath.Join(NodePath(name), "config.json")
	return renvm.NewOptionsFromFile(path)
}

// NodeIP gets the IP address of the node with given name.
func NodeIP(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	cmd := fmt.Sprintf("cd %v && terraform output ip", NodePath(name))
	ip, err := CommandOutput(cmd)
	if err != nil {
		return "", err
	}
	if strings.Contains(ip, "Warning") {
		return "", fmt.Errorf("no ouput ip")
	}
	if strings.HasPrefix(ip, "\"") {
		return strings.Trim(strings.TrimSpace(ip), "\""), err
	}
	return strings.TrimSpace(ip), nil
}

// NodeEthereumAddr gets the ethereum address of the node with given name.
func NodeEthereumAddr(pk *id.PrivKey) common.Address {
	return crypto.PubkeyToAddress(pk.PublicKey)
}

// NodeProvider returns the provider of the node with given name.
func NodeProvider(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	cmd := fmt.Sprintf("cd %v && terraform output provider", NodePath(name))
	provider, err := CommandOutput(cmd)
	if strings.HasPrefix(provider, "\"") {
		provider = strings.Trim(strings.TrimSpace(provider), "\"")
	}
	return strings.TrimSpace(provider), err
}

// GetNodesByTags return the names of the nodes which have the given tags.
func GetNodesByTags(tags string) ([]string, error) {
	files, err := ioutil.ReadDir(filepath.Join(Directory, "darknodes"))
	if err != nil {
		return nil, err
	}
	nodes := make([]string, 0)
	for _, f := range files {
		path := filepath.Join(Directory, "darknodes", f.Name(), "tags.out")
		tagFile, err := ioutil.ReadFile(path)
		if err != nil {
			// If the tags.out file doesn't exist, use empty tags.
			tagFile = []byte{}
		}
		if !ValidateTags(string(tagFile), tags) {
			continue
		}
		nodes = append(nodes, f.Name())
	}
	if len(nodes) == 0 {
		return nil, errors.New("cannot find any node with given tags")
	}

	return nodes, nil
}

func ValidateTags(have, required string) bool {
	tagsStr := strings.Split(strings.TrimSpace(required), ",")
	for _, tag := range tagsStr {
		if !strings.Contains(have, tag) {
			return false
		}
	}
	return true
}
