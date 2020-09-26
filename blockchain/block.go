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

func (bh *BlockHeader) Copy() *BlockHeader {
    prevBlock := make(BlockID, len(bh.PrevBlock))
    copy(prevBlock, bh.PrevBlock)
    merkleRoot := make(BlockID, len(bh.MerkleRoot))
    copy(merkleRoot, bh.MerkleRoot)
    data := make([]byte, len(bh.Data))
    copy(data, bh.Data)
    return &BlockHeader{
        Index:		bh.Index,
	Version:	bh.Version,
	Bits:		bh.Bits,
	Nonce:		bh.Nonce,
	Timestamp:	bh.Timestamp,
	PrevBlock:	prevBlock,
	MerkleRoot:	merkleRoot,
	Data:		data,
    }
}

type Block struct {
    *BlockHeader
    Hash []byte
    // Public Key
    PublicKey string
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
    // hash.Write([]byte(b.PublicKey))
    buf := hash.Sum(nil)
    return buf, nil
}

// Copy makes a deep copy of the Block
func (b *Block) Copy() *Block {
    if b == nil {
        return nil
    }
    block := &Block{
        BlockHeader:	b.BlockHeader.Copy(),
	Hash:		make([]byte, len(b.Hash)),
	Payload:	make([]byte, len(b.Payload)),
	Transactions:	make([]*Transaction, len(b.Transactions)),
    }
    copy(block.Hash, b.Hash)
    copy(block.Payload, b.Payload)
    return block
}
