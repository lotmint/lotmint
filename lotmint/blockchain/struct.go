package blockchain

import (
    "go.dedis.ch/onet/v3/network"
)

// BlockID represents the Hash of the Block
type BlockID []byte

func init() {
    network.RegisterMessage(&BlockHeader{})
    network.RegisterMessage(&Block{})
}
