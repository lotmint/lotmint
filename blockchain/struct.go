package blockchain

import (
    "go.dedis.ch/onet/v3/network"
)

const DEFAULT_BITS = 0x1e016800

// BlockID represents the Hash of the Block
type BlockID []byte

func init() {
    network.RegisterMessage(&BlockHeader{})
    network.RegisterMessage(&Block{})
}
