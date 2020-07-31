package service

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
    PrevBlock string
    // Merkle tree reference to hash of all transactions for the block.
    MerkleRoot string
}

type Block struct {
    *BlockHeader
    Hash string
}

type KeyBlock struct {
    *Block
}

type TxBlock struct {
    *Block
    Txs []*Transaction
}

func NewKeyBlock() *KeyBlock {
    return &KeyBlock{}
}
