package renvm

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
	"github.com/renproject/multichain"
	"github.com/renproject/pack"
	"github.com/renproject/surge"
)

type State []Contract

type Contract struct {
	Address pack.String `json:"address"`
	State   pack.Typed  `json:"state"`
}

// SystemState defines the state of the "System" contract. It records
// information about RenVM itself, including shards, epochs, and so on.
type SystemState struct {
	Epoch  SystemStateEpoch  `json:"epoch"`
	Nodes  []SystemStateNode `json:"nodes"`
	Shards SystemStateShards `json:"shards"`
}

// SystemStateEpoch defines the record of the current epoch. It includes the
// epoch hash and number.
type SystemStateEpoch struct {
	Hash      pack.Bytes32 `json:"hash"`
	Number    pack.U64     `json:"number"`
	NumNodes  pack.U64     `json:"numNodes"`
	Timestamp pack.U64     `json:"timestamp"`
}

// SystemStateNode defines the record of a single node. Currently, it includes
// the public key of the node.
type SystemStateNode struct {
	ID        pack.Bytes32 `json:"id"`
	EnteredAt pack.U64     `json:"enteredAt"`
}

// SystemStateShards defines the record of shards kept by the "System" contract.
// At the beginning of every epoch, new primary shards are selected, primary
// shards become secondary shards, secondary shards become tertiary shards, and
// tertiary shards are dropped.
type SystemStateShards struct {
	// Primary shards are used by all new cross-chain transactions.
	Primary []SystemStateShardsShard `json:"primary"`
	// Secondary shards finish processing the remaining cross-chain transactions
	// that are left over from when these shards were primary shards. This
	// overlap protects against cross-chain transactions getting lost between
	// epochs.
	Secondary []SystemStateShardsShard `json:"secondary"`
	// Tertiary shards do nothing. They exist to allow time for malicious nodes
	// in the shards to be punished for malicious behaviour that may have
	// occurred when these shards where primary/secondary shards.
	Tertiary []SystemStateShardsShard `json:"tertiary"`
}

// SystemStateShardsShard defines the record of one shard. It includes the
// identify of the shard, and the ECDSA public key of the shard (encoded using
// the 33-byte compressed format).
type SystemStateShardsShard struct {
	Shard  pack.Bytes32 `json:"shard"`
	PubKey pack.Bytes   `json:"pubKey"`
}

func NewGenesis(network multichain.Network, peers []wire.Address) (State, error) {
	if len(peers) == 0 {
		return State{}, fmt.Errorf("fetching signatory : empty darknode address")
	}
	shardSignatory, err := peers[0].Signatory()
	if err != nil {
		return State{}, err
	}
	shardHash := id.NewMerkleHashFromSignatories([]id.Signatory{shardSignatory})

	pubkey := RenVMPubKey(network)
	renVMPubKey := (*id.PubKey)(pubkey)
	renVMPubKeyBytes, err := surge.ToBinary(renVMPubKey)
	if err != nil {
		return State{}, err
	}

	systemState := SystemState{
		Epoch: SystemStateEpoch{},
		Nodes: []SystemStateNode{},
		Shards: SystemStateShards{
			Primary: []SystemStateShardsShard{
				{
					Shard:  pack.Bytes32(shardHash),
					PubKey: renVMPubKeyBytes,
				},
			},
			Secondary: []SystemStateShardsShard{},
			Tertiary:  []SystemStateShardsShard{},
		},
	}
	systemStateEncoded, err := pack.Encode(systemState)
	if err != nil {
		panic(fmt.Sprintf("encoding state: %v", err))
	}
	return State{
		{
			Address: "System",
			State:   pack.Typed(systemStateEncoded.(pack.Struct)),
		},
	}, nil
}

func NewGenesisFromFile(path string) (State, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	genesisFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer genesisFile.Close()

	var genState State
	err = json.NewDecoder(genesisFile).Decode(&genState)
	return genState, err
}

// Compressed RenVM public key in hex encoding
func RenVMPubKey(network multichain.Network) *ecdsa.PublicKey {
	var pubKeyStr string
	switch network {
	case multichain.NetworkDevnet:
		pubKeyStr = "0232938bc9b3fd09488a77704d102f769b6c89cd33de4b72d309f83bf1b7e26f50"
	case multichain.NetworkTestnet:
		pubKeyStr = "030dd65f7db2920bb229912e3f4213dd150e5f972c9b73e9be714d844561ac355c"
	case multichain.NetworkMainnet:
		pubKeyStr = "038927457800931c7d44dc94cb0021b4e881f9935c1234dc92f456e32ec019d5d7"
	default:
		panic(fmt.Sprintf("unsupport network = %v", network))
	}

	// Decode the hex string and get the RenVM public key
	keyBytes, err := hex.DecodeString(pubKeyStr)
	if err != nil {
		panic(fmt.Sprintf("invalid public key string from the env variable, err = %v", err))
	}
	key, err := crypto.DecompressPubkey(keyBytes)
	if err != nil {
		panic(fmt.Sprintf("invalid distribute public key, err = %v", err))
	}
	return key
}
