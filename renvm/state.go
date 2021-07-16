package renvm

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/aw/wire"
	"github.com/renproject/id"
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

func NewGenesis(peers []wire.Address) (State, error) {
	if len(peers) == 0 {
		return State{}, fmt.Errorf("fetching signatory : empty darknode address")
	}
	shardSignatory, err := peers[0].Signatory()
	if err != nil {
		return State{}, err
	}
	shardHash := id.NewMerkleHashFromSignatories([]id.Signatory{shardSignatory})

	// We have the assumption the last peer is the execution node
	pubkey, err := AddrToPub(peers[len(peers)-1])
	if err != nil {
		return State{}, err
	}
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

// AddrToPub extracts the public key from the given address. The address must
// be signed, otherwise an error will be returned.
func AddrToPub(addr wire.Address) (*ecdsa.PublicKey, error) {
	zeroSig := id.Signature{}
	if addr.Signature.Equal(&zeroSig) {
		return nil, fmt.Errorf("address not signed")
	}
	buf := make([]byte, surge.SizeHintU8+surge.SizeHintString(addr.Value)+surge.SizeHintU64)
	hash, err := wire.NewAddressHashWithBuffer(addr.Protocol, addr.Value, addr.Nonce, buf)
	if err != nil {
		return nil, err
	}
	return crypto.SigToPub(hash[:], addr.Signature[:])
}