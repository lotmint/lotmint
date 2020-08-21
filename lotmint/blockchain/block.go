package blockchain

import (
    "crypto/sha256"
    "encoding/binary"
    "errors"
)

type BlockHeader struct {
    // Index of the block in the chain. Index = 0 -> genesis-block.
    Index int
    // Version of the block.
    Version uint32
    // Difficulty target for the block.
    Bits uint32
    // Nonce used to generate the block.
    Nonce uint64
    // Time the block was created.
    Timestamp uint64
    // Hash of the previous block header in the block chain.
    PrevBlock BlockID
    // Merkle tree reference to hash of all transactions for the block.
    MerkleRoot BlockID
    // Data is any data to be stored in that Block.
    Data []byte
}

type Block struct {
    *BlockHeader
    Hash []byte
    // Payload is additional data that needs to be hashed by the application
    // itself into BlockHeader.Data.
    Payload []byte `protobuf:"opt"`
    Transactions []*Transaction
}

func NewBlock() *Block {
    return &Block{
        BlockHeader: &BlockHeader{
            Data: make([]byte, 0),
	},
    }
}

// CalculateHash hashes all block header of the block.
func (b *Block) CalculateHash() (BlockID, error) {
    hash := sha256.New()
    err := binary.Write(hash, binary.LittleEndian, int32(b.Index))
    if err != nil {
        return nil, errors.New("error writing to hash:" + err.Error())
    }
    for _, val := range []uint32{b.Version, b.Bits} {
        err = binary.Write(hash, binary.LittleEndian, val)
        if err != nil {
            return nil, errors.New("error writing to hash:" + err.Error())
        }
    }
    for _, val := range []uint64{b.Nonce, b.Timestamp} {
        err = binary.Write(hash, binary.LittleEndian, val)
        if err != nil {
            return nil, errors.New("error writing to hash:" + err.Error())
        }
    }

    hash.Write(b.PrevBlock)
    hash.Write(b.MerkleRoot)
    hash.Write(b.Data)
    buf := hash.Sum(nil)
    return buf, nil
}
