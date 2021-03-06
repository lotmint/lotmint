package blockchain

import (
    "crypto/sha256"
    "encoding/binary"
    "encoding/hex"
    "errors"
    "fmt"
    "strings"
    "time"

	"go.dedis.ch/onet/v3/network"
)

type BlockHeader struct {
    // Index of the block in the chaign. Index = 0 -> genesis-block.
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

    // Public Key
    PublicKey string

    // IP Addresses
    Addresses []*network.ServerIdentity

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
    var addresses []*network.ServerIdentity
    for _, addr := range bh.Addresses {
        addresses = append(addresses, addr) // Copy
    }
    return &BlockHeader{
        Index:		bh.Index,
	    Version:	bh.Version,
	    Bits:		bh.Bits,
	    Nonce:		bh.Nonce,
	    Timestamp:	bh.Timestamp,
	    PrevBlock:	prevBlock,
	    MerkleRoot:	merkleRoot,
        PublicKey:  bh.PublicKey,
        Addresses:  addresses,
	    Data:		data,
    }
}

type Block struct {
    *BlockHeader
    Hash []byte
    Transactions []*Transaction
    OrderBlocks []*Block
}

func NewBlock() *Block {
    return &Block{
        BlockHeader: &BlockHeader{
            Data: make([]byte, 0),
	    },
	    Hash: make([]byte, 0),
        Transactions: make([]*Transaction, 0),
        OrderBlocks: make([]*Block, 0),
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
    hash.Write([]byte(b.PublicKey))
    hash.Write(b.Data)
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
	    Hash:           make([]byte, len(b.Hash)),
	    Transactions:	make([]*Transaction, len(b.Transactions)),
        OrderBlocks:    make([]*Block, len(b.OrderBlocks)),
    }
    copy(block.Hash, b.Hash)
    for _, tx := range b.Transactions {
        block.Transactions = append(block.Transactions, tx) // tx.Copy()
    }
    for _, ob := range b.OrderBlocks {
        block.OrderBlocks = append(block.OrderBlocks, ob.Copy())
    }
    return block
}

func (b *Block) String() string {
    var builder strings.Builder
    builder.WriteString(fmt.Sprintf("Block %d", b.Index))
    builder.WriteString(fmt.Sprintf("\n\tHeight: %d", b.Index))
    builder.WriteString(fmt.Sprintf("\n\tVersion: %d", b.Version))
    builder.WriteString(fmt.Sprintf("\n\tBits: 0x%x", b.Bits))
    builder.WriteString(fmt.Sprintf("\n\tNonce: %d", b.Nonce))
    builder.WriteString(fmt.Sprintf("\n\tTimestamp: %s", time.Unix(0, int64(b.Timestamp)).Format("2006-01-02 15:04:05")))
    builder.WriteString(fmt.Sprintf("\n\tPrevBlock: %s", hex.EncodeToString(b.PrevBlock)))
    builder.WriteString(fmt.Sprintf("\n\tMerkleRoot: %s", hex.EncodeToString(b.MerkleRoot)))
    builder.WriteString(fmt.Sprintf("\n\tPublicKey: %s", b.PublicKey))
    builder.WriteString(fmt.Sprintf("\n\tAddresses: %v", b.Addresses))
    builder.WriteString(fmt.Sprintf("\n\tData: %s", hex.EncodeToString(b.Data)))
    builder.WriteString(fmt.Sprintf("\n\tHash: %s", hex.EncodeToString(b.Hash)))
    return builder.String()
}
