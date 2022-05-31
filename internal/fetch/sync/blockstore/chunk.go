package blockstore

import (
	"fmt"
	"path/filepath"
	"strconv"

	"gitlab.com/thorchain/midgard/internal/util/miderr"
)

type chunk struct {
	name   string
	height int64
	hash   string
}

const currentChunkName = "tmp"
const withoutExtension = ""

func NewChunk(name string) (*chunk, error) {
	t := chunk{name: name}
	mh, err := t.maxHeight()
	t.height = mh
	return &t, err
}

func (r chunk) localPath(blockStore *BlockStore) string {
	return filepath.Join(blockStore.cfg.Local, r.name)
}

func (r chunk) remotePath(b *BlockStore) string {
	return b.cfg.Remote + r.name
}

func (r chunk) maxHeight() (int64, error) {
	height, err := r.toHeight()
	if err != nil {
		return 0, miderr.InternalErrF("BlockStore: cannot convert chunk %v", r)
	}
	return height, nil
}

func (r chunk) toHeight() (int64, error) {
	return strconv.ParseInt(r.name, 10, 64)
}

func toChunk(height int64) chunk {
	return chunk{name: fmt.Sprintf("/%012d", height)}
}
