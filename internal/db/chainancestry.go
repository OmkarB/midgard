package db

import (
	"sync/atomic"
	"unsafe"

	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/midgard/config"
)

var wellKnownChainInfos = []config.ForkInfo{
	// Mainnet
	{ChainId: "thorchain", EarliestBlockHash: "7D37DEF6E1BE23C912092069325C4A51E66B9EF7DDBDE004FF730CFABC0307B1", EarliestBlockHeight: 1, HardForkHeight: 4786559},
	{ChainId: "thorchain-mainnet-v1", ParentChainId: "thorchain", EarliestBlockHeight: 4786560},

	// Testnet
	{ChainId: "thorchain-testnet-v0", EarliestBlockHash: "D4DF73AD98535DCD72BD0C9FE76B96CAF350C2FF517A61F77F5F89665A0593E7", EarliestBlockHeight: 1, HardForkHeight: 1276571},
	{ChainId: "thorchain-v1", ParentChainId: "thorchain-testnet-v0", EarliestBlockHash: "771423E3B5F15BBA164BB54E0CD654FBC050494D98AC04A66C207494653A958D", EarliestBlockHeight: 1276572, HardForkHeight: 1821177},
	{ChainId: "thorchain-testnet-v2", ParentChainId: "thorchain-v1", EarliestBlockHash: "107C3BA9DB7952FF683A59D559216800B7A4E9AB8584EBF7456F55AA5516C33A", EarliestBlockHeight: 1821178},
}

var mergedChainMap unsafe.Pointer

func CombinedForkInfoMap() *map[string]config.ForkInfo {
	merged := (*map[string]config.ForkInfo)(atomic.LoadPointer(&mergedChainMap))
	if merged != nil {
		return merged
	}

	m := make(map[string]config.ForkInfo)
	infos := []config.ForkInfo{}
	infos = append(infos, wellKnownChainInfos...)
	infos = append(infos, config.Global.ThorChain.ForkInfos...)

	for _, fi := range infos {
		if fi.ParentChainId != "" {
			parent, ok := m[fi.ParentChainId]
			if !ok {
				log.Fatal().Msgf("Chain '%s' has parent '%s', but it's not defined",
					fi.ChainId, fi.ParentChainId)
			}
			if parent.HardForkHeight == 0 {
				log.Fatal().Msgf("Chain '%s' is a parent of '%s', but has no HardForkHeight defined",
					fi.ParentChainId, fi.ChainId)
			}
			if fi.EarliestBlockHeight == 0 {
				fi.EarliestBlockHeight = parent.HardForkHeight + 1
			}
			if fi.EarliestBlockHeight != parent.HardForkHeight+1 {
				log.Fatal().Msgf("Height discontinuity: %s ends at %d, %s starts at %d",
					fi.ParentChainId, parent.HardForkHeight, fi.ChainId, fi.EarliestBlockHeight)
			}
		} else {
			if fi.EarliestBlockHeight == 0 {
				fi.EarliestBlockHeight = 1
			}
		}
		if fi.HardForkHeight != 0 && fi.HardForkHeight < fi.EarliestBlockHeight {
			log.Fatal().Msgf(
				"Invalid ForkInfo for '%s': HardForkHeight[=%d] < EarliestBlockHeight[=%d]",
				fi.ChainId, fi.HardForkHeight, fi.EarliestBlockHeight)
		}

		m[fi.ChainId] = fi
	}

	atomic.StorePointer(&mergedChainMap, unsafe.Pointer(&m))
	return &m
}

func mergeAdditionalInfo(chainId *FullyQualifiedChainId, info config.ForkInfo) {
	if info.EarliestBlockHash != "" {
		chainId.StartHash = info.EarliestBlockHash
	}
	if info.EarliestBlockHeight != 0 {
		chainId.StartHeight = info.EarliestBlockHeight
	}
	if info.HardForkHeight != 0 {
		chainId.HardForkHeight = info.HardForkHeight
	}
}

func EnrichAndGetRoot(chainId *FullyQualifiedChainId) FullyQualifiedChainId {
	m := *CombinedForkInfoMap()
	target, ok := m[chainId.Name]
	if !ok {
		if chainId.StartHeight != 1 {
			log.Fatal().Msgf(
				`Chain '%s' does not start at 1, yet it doesn't have a ForkInfo definition
				If you intend to start a truncated chain, add a ForkInfo definition for it without
				specifying a parent`, chainId.Name)
		}
		return *chainId
	}

	mergeAdditionalInfo(chainId, target)
	if chainId.HardForkHeight != 0 && chainId.HardForkHeight < chainId.StartHeight {
		log.Fatal().Msgf(
			"Merging in ForkInfo resulted in invalid data for chain '%s': HardForkHeight[=%d] < StartHeight[=%d]",
			chainId.Name, chainId.HardForkHeight, chainId.StartHeight)
	}

	for {
		parent, ok := m[target.ParentChainId]
		if !ok {
			break
		}
		target = parent
	}

	if target.ChainId == chainId.Name {
		return *chainId
	}
	return FullyQualifiedChainId{
		Name:           target.ChainId,
		StartHash:      target.EarliestBlockHash,
		StartHeight:    target.EarliestBlockHeight,
		HardForkHeight: target.HardForkHeight,
	}
}
