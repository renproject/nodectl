package util

import (
	"fmt"

	"github.com/renproject/multichain"
)

// GetFileURL returns the URL of the requested file in s3, the network input
// must be a valid network, otherwise function will panic.
func GetFileURL(network multichain.Network, name string) string {
	switch network {
	case multichain.NetworkMainnet, multichain.NetworkTestnet, multichain.NetworkDevnet:
		return fmt.Sprintf("https://s3.ap-southeast-1.amazonaws.com/darknode.renproject.io/%v/%v", network, name)
	default:
		panic("invalid network")
	}
}

// GenesisURL returns the url of the genesis file on the given network.
func GenesisURL(network multichain.Network) string {
	return GetFileURL(network, "genesis.json")
}

// OptionsURL returns the url of the options template.
func OptionsURL(network multichain.Network) string {
	return GetFileURL(network, "config.json")
}

// SnapshotURL returns the url of the latest snapshot file on the given network.
func SnapshotURL(network multichain.Network, name string) string {
	if name == ""{
		name = "latest.tar.gz"
	}
	return GetFileURL(network, name)
}
