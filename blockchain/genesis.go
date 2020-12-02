package blockchain

import (
    "lotmint/utils"

    "go.dedis.ch/onet/v3/network"
)


// genesisMerkleRoot is the hash of the first transaction in the genesis block
// for the main network.
var genesisMerkleRoot = BlockID([]byte{
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
})

// genesisHash is the hash of the first block int the block chain for the main
// network (genesis block).
var genesisHash = BlockID([]byte{
    0x8d, 0x15, 0x7a, 0xca, 0x50, 0x1e, 0x91, 0x61,
    0x68, 0x9a, 0x05, 0x61, 0xe0, 0x78, 0x2e, 0x13,
    0xe9, 0x41, 0xc0, 0xec, 0x0e, 0x25, 0x28, 0x2c,
    0x88, 0xd7, 0xbe, 0xcd, 0x5c, 0x94, 0xb6, 0x56,
})

var genesisAddresses = [...]string{
    "tls://b2aa3a0faf75e5b09f048a30361a41d380c999531dd64123ba699fa4b9bcdcb7@127.0.0.1:7770",
}

func GetGenesisBlock() *Block{
    var addresses []*network.ServerIdentity
    for _, address := range genesisAddresses {
        si, err := utils.ConvertPeerURL(address)
        if err != nil {
        }
        addresses = append(addresses, si)
    }
    // genesisBlock defines the genesis block of the block chain which serves as the
    // public transaction ledger for the main network.
    var genesisBlock = &Block{
        BlockHeader: &BlockHeader{
                Index:		0,
                Version:	1,
                Bits:		DEFAULT_BITS,
                Nonce:		0x1343d72,	// 20200818
                PrevBlock:	BlockID{},	// 0000000000000000000000000000000000000000000000000000000000000000
                Timestamp:	1597680000,	// 2020-08-18
                MerkleRoot:	genesisMerkleRoot,
                PublicKey:  "b2aa3a0faf75e5b09f048a30361a41d380c999531dd64123ba699fa4b9bcdcb7",
                Addresses:  addresses,
                Data:		make([]byte, 0),
            },
            Hash:		genesisHash,
            Transactions:	make([]*Transaction, 0),
    }
    return genesisBlock
}
