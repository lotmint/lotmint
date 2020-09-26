package blockchain

import (
    "testing"
)

func TestNewGenesisBlock(t *testing.T) {
    hash, err := genesisBlock.CalculateHash()
    if err != nil {
        t.Errorf(err.Error())
    } else {
        t.Errorf("Genesis Block hash: %#x", hash)
    }
}
