package renvm

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
)

var (
	ConfigURLTestnet = "https://s3.ap-southeast-1.amazonaws.com/darknode.renproject.io/testnet.json"

	ConfigURLMainnet = "s3://darknode.renproject.io/mainnet.json"
)

// Default options.
var (
	DefaultHome     = "/home/darknode/.darknode"
	DefaultHost     = "0.0.0.0"
	DefaultPort     = uint16(18514)
	DefaultSimulate = Simulate{
		SimulateOutOfMemory: SimulateOutOfMemory{
			Enable: false,
		},
	}
	DefaultProfile = false
)

// Options parameterise the behaviour of a node. During testing, options are
// usually defined in-memory, however these options can also be
// marshalled/unmarshalled to/from the disk. This allows the node to keep the
// exact same configuration between boots, and allows the operator to modify its
// behaviour at runtime (by modifying the configuration file, and then rebooting
// the node).
type Options struct {

	// Home directory for the database and configuration files.
	Home string `json:"home"`

	// PrivKey that defines the identity of the node.
	PrivKey *id.PrivKey `json:"privKey"`

	// Peers that the node will use to bootstrap into the network (in addition
	// to peers that have been saved to the database).
	Peers []wire.Address `json:"peers"`

	// Host that will be used to listen for incoming connections.
	Host string `json:"host"`
	// Port that will be used to listen for incoming connections.
	Port uint16 `json:"port"`

	// Simulate enables/disables different simulation options. These options
	// should only be used during development and testing.
	Simulate Simulate `json:"simulate"`

	// Profile enables/disables the exposure of profiling information at
	// http://localhost:8080.
	Profile bool `json:"profile"`

	// Network environment. Must be one of "localnet", "testnet", or "mainnet".
	Network multichain.Network `json:"net"`

	// Chains along with their options.
	Chains map[multichain.Chain]ChainOptions `json:"chains"`

	// Selectors is a whitelist containing the supported selectors.
	Selectors []Selector `json:"selectors"`
}

// Simulate defines options for enabling/configuring the simulation of test
// scenarios that are difficult to force externally.
type Simulate struct {
	SimulateOutOfMemory `json:"oom"`
}

// SimulateOutOfMemory defines options for enabling/configuring the simulation
// of out-of-memory errors.
type SimulateOutOfMemory struct {
	Enable bool `json:"enable"`
	Min    int  `json:"min"`
	Max    int  `json:"max"`
}

// ChainOptions used to parameterise chain-specific behaviour for chains that
// are supported by the transaction engine bindings. It is expected that these
// options will be marshalled to/from JSON as part of configuring the overall
// system.
type ChainOptions struct {
	RPC              pack.String                 `json:"rpc"`
	Confirmations    pack.U64                    `json:"confirmations"`
	MaxConfirmations pack.U64                    `json:"maxConfirmations"`
	GasLimit         pack.U256                   `json:"gasLimit"`
	Protocol         pack.String                 `json:"protocol"`
	Fees             map[multichain.Chain]Fees   `json:"fees"`
	Extras           map[pack.String]pack.String `json:"extras"`
}

type Fees struct {
	MintFee pack.U64 `json:"mintFee"`
	BurnFee pack.U64 `json:"burnFee"`
}

func NewOptions(network multichain.Network) Options {
	return Options{
		Home:     DefaultHome,
		PrivKey:  id.NewPrivKey(),
		Host:     DefaultHost,
		Port:     DefaultPort,
		Simulate: DefaultSimulate,
		Profile:  DefaultProfile,
		Network:  network,
	}
}

func NewOptionsFromFile(path string) (Options, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Options{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return Options{}, err
	}
	defer file.Close()
	var opts Options
	err = json.NewDecoder(file).Decode(&opts)
	return opts, err
}

func OptionTemplate(url string) (Options, error) {
	response, err := http.Get(url)
	if err != nil {
		return Options{}, err
	}
	defer response.Body.Close()

	var opts Options
	if err := json.NewDecoder(response.Body).Decode(&opts); err != nil {
		return Options{}, err
	}
	return opts, nil
}
