package blockstore

import (
	tmjson "github.com/tendermint/tendermint/libs/json"
	"gitlab.com/thorchain/midgard/internal/fetch/sync/chain"
	"gitlab.com/thorchain/midgard/internal/util/midlog"
)

// Wrapper for chain.Block that makes it possible to serialize with generic libraries
//
// Handles error="gob: type not registered for interface: type ..." serialization errors
//
// Affected fields:
//	chain.Block.Results.ValidatorUpdates
//
// Details:
//   Most serialization libraries (and the CBOR library that we use, in particular) cannot handle
//   `chain.Block.Results.ValidatorUpdates`, as it contains an interface field. Fortunately,
//   most of the time `ValidatorUpdates` is nil, so we don't have any issues. Whenever it's not
//   nil, we serialize it separately, by its native tendermint library and store it separately
//   like that.
type storedBlock struct {
	Block                      chain.Block
	SerializedValidatorUpdates []byte
	SerializedEvidence         []byte
}

func blockToStored(block *chain.Block) (*storedBlock, error) {
	sBlock := storedBlock{Block: *block}

	if sBlock.Block.Results.ValidatorUpdates != nil {
		serialized, err := tmjson.Marshal(sBlock.Block.Results.ValidatorUpdates)
		if err != nil {
			return nil, err
		}
		sBlock.SerializedValidatorUpdates = serialized

		// Decouple the storedBlock from the input, so we can update it.
		// (Only the parts necessary for the update are copied.)
		copy0 := *sBlock.Block.Results
		sBlock.Block.Results = &copy0

		// Remove the field that must be serialized separately
		sBlock.Block.Results.ValidatorUpdates = nil
	}

	return &sBlock, nil
}

// Note: mutates the supplied argument!
func storedToBlock(sBlock *storedBlock) (*chain.Block, error) {
	if sBlock.SerializedValidatorUpdates != nil {
		if sBlock.Block.Results == nil {
			midlog.Fatal("Empty results in stored block")
		}
		err := tmjson.Unmarshal(sBlock.SerializedValidatorUpdates,
			&sBlock.Block.Results.ValidatorUpdates)
		if err != nil {
			return nil, err
		}
	}
	return &sBlock.Block, nil
}
