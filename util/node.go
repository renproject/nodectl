package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
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
		return []string{name}, ValidateNodeName(name)
	} else {
		return nil, ErrTooManyArguments
	}
}

// ValidateNodeName checks if there exists a node with given name.
func ValidateNodeName(name string) error {
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

// // Config returns the config of the node with given name.
// func Config(name string) (renvm.GeneralConfig, error) {
// 	path := filepath.Join(NodePath(name), "config.json")
// 	return renvm.NewConfigFromJSONFile(path)
// }
//
// // ID gets the ID of the node with given name.
// func ID(name string) (addr.ID, error) {
// 	path := filepath.Join(NodePath(name), "config.json")
// 	config, err := renvm.NewConfigFromJSONFile(path)
// 	if err != nil {
// 		return addr.ID{}, err
// 	}
// 	return addr.FromPublicKey(config.Keystore.Ecdsa.PublicKey), nil
// }

// NodeIP gets the IP address of the node with given name.
func NodeIP(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	cmd := fmt.Sprintf("cd %v && terraform output ip", NodePath(name))
	ip, err := CommandOutput(cmd)
	return strings.TrimSpace(ip), err
}

// NodeProvider returns the provider of the node with given name.
func NodeProvider(name string) (string, error) {
	if name == "" {
		return "", ErrEmptyName
	}

	cmd := fmt.Sprintf("cd %v && terraform output provider", NodePath(name))
	provider, err := CommandOutput(cmd)
	return strings.TrimSpace(provider), err
}

// Version gets the version of the software the darknode currently is running.
func Version(name string) string {
	script := "cat ~/.darknode/version"
	version, err := RemoteOutput(name, script)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(version))
}

// GetNodesByTags return the names of the nodes which have the given tags.
func GetNodesByTags(tags string) ([]string, error) {
	files, err := ioutil.ReadDir(filepath.Join(Directory, "/nodes"))
	if err != nil {
		return nil, err
	}
	nodes := make([]string, 0)
	for _, f := range files {
		path := filepath.Join(Directory, "nodes", f.Name(), "tags.out")
		tagFile, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}
		if !ValidateTags(string(tagFile), tags) {
			continue
		}
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
