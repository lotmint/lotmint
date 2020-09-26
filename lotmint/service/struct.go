package service

import (
    "math/rand"
    "sync"

    bc "lotmint/blockchain"
    "lotmint/blscosi"

    bbolt "go.etcd.io/bbolt"
    "go.dedis.ch/onet/v3/log"
    "go.dedis.ch/onet/v3/network"
    "golang.org/x/xerrors"
)

type BlockID bc.BlockID

type txPool struct {
    sync.Mutex
    txsMap map[string][]bc.Transaction
}

func newTxPool() txPool {
    return txPool{
        txsMap: make(map[string][]bc.Transaction),
    }
}

func (p *txPool) add(key string, tx bc.Transaction) {
    p.Lock()
    defer p.Unlock()

    if txs, ok := p.txsMap[key]; !ok {
	p.txsMap[key] = []bc.Transaction{tx}
    } else {
	txs = append(txs, tx)
	p.txsMap[key] = txs
    }
}

// Announce is used to pass a message to all children.
type ServiceMessage struct {
    Message string
}

type BlockMessage struct {
    Type int // 0:CandidateBlock,1:RefererBlock,2:TxBlock
    Block *bc.Block
}

type HandshakeMessage struct {
    GenesisID BlockID
    LatestBlock *bc.Block
    Answer  bool
}

type DownloadBlockRequest struct {
    GenesisID BlockID
    Start int
    Size int
}

type DownloadBlockResponse struct {
    Blocks []*bc.Block
    GenesisID BlockID
}

// SignatureRequest is what the Cosi service is expected to receive from clients.
type SignatureRequest struct {
    // ServerIdentity	*network.ServerIdentity
    Message []byte
    // Roster  *onet.Roster
}

// SignatureResponse is what the Cosi service will reply to clients.
type SignatureResponse struct {
    Hash      []byte
    Responses []*blscosi.Response
    // Signature protocol.BlsSignature
}

type RemoteServerIndex struct {
    Index int
    ServerIdentity *network.ServerIdentity
}

type BlockDB struct {
    *bbolt.DB
    bucketName []byte
    latestBlockID BlockID
    latestMutex sync.Mutex
    blockIndexMap map[int]BlockID
    // Cached previous N referer blocks for searching quickly
    refererBlocks []*bc.Block
    refererMutex sync.Mutex
}

// NewBlockDB returns an initialized BlockDB structure.
func NewBlockDB(db *bbolt.DB, bn[]byte) *BlockDB {
	return &BlockDB{
		DB: db,
		bucketName: bn,
		blockIndexMap: make(map[int]BlockID),
		refererBlocks: make([]*bc.Block, 0),
	}
}

// storeToTx stores the block into the database.
func (db *BlockDB) storeToTx(tx *bbolt.Tx, block *bc.Block) (BlockID, error) {
        log.Lvlf2("Storing block %d / %x", block.Index, block.Hash)
	val, err := network.Marshal(block)
	if err != nil {
		return nil, err
	}
	return block.Hash, tx.Bucket([]byte(db.bucketName)).Put(block.Hash, val)
}

// getFromTx returns the skipblock identified by sbID.
// nil is returned if the key does not exist.
func (db *BlockDB) getFromTx(tx *bbolt.Tx, sbID BlockID) (*bc.Block, error) {
	if sbID == nil {
		return nil, xerrors.New("cannot look up block with ID == nil")
	}

	val := tx.Bucket(db.bucketName).Get(sbID)
	if val == nil {
		return nil, nil
	}

	// For some reason boltdb changes the val before Unmarshal finishes. When
	// copying the value into a buffer, there is no SIGSEGV anymore.
	buf := make([]byte, len(val))
	copy(buf, val)
	_, sbMsg, err := network.Unmarshal(buf, suite)
	if err != nil {
		return nil, err
	}

	return sbMsg.(*bc.Block).Copy(), nil
}

// Stores the set of blocks in the boltdb
func (db *BlockDB) StoreBlocks(blocks []*bc.Block) ([]BlockID, error) {
    var result []BlockID
    err := db.Update(func(tx *bbolt.Tx) error {
        for _, block := range blocks {
	   blockID, err := db.storeToTx(tx, block)
	   if err != nil {
	        return err
	   }
	   result = append(result, blockID)
	}
        return nil
    })
    return result, err
}

// Stores the given Block into db
func (db *BlockDB) Store(block *bc.Block) BlockID {
	ids, err := db.StoreBlocks([]*bc.Block{block})
	if err != nil {
		log.Error(err)
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return nil
}

// GetGenesisID
func (db *BlockDB) GetGenesisID() (BlockID) {
    blockID, ok := db.blockIndexMap[0]
    if !ok {
	log.Error("no genesis block found")
        return nil
    }
    return blockID
}


// GetLatest searches for the latest available block.
func (db *BlockDB) GetLatest() (*bc.Block) {
    db.latestMutex.Lock()
    latest := db.GetByID(db.latestBlockID)
    db.latestMutex.Unlock()
    return latest
}

func (db *BlockDB) UpdateLatest(blockID bc.BlockID) {
    db.latestMutex.Lock()
    db.latestBlockID = BlockID(blockID)
    db.latestMutex.Unlock()
}

// GetByID returns a new copy of the skip-block or nil if it doesn't exist
func (db *BlockDB) GetByID(bID BlockID) *bc.Block {
    var result *bc.Block
    if bID == nil {
        return nil
    }
    err := db.View(func(tx *bbolt.Tx) error {
        block, err := db.getFromTx(tx, bID)
	if err != nil {
	    return err
	}
	result = block
	return nil
    })

    if err != nil {
        log.Error(err)
    }
    return result
}

// Build block index
func (db *BlockDB) BuildIndex() error {
    return db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket(db.bucketName)
        if b == nil {
            return xerrors.New("Missing bucket")
        }
        return b.ForEach(func(k []byte, v []byte) error {
            _, msg, err := network.Unmarshal(v, suite)
	    if err != nil {
                return err
	    }
	    block := msg.(*bc.Block).Copy()
	    log.Lvlf3("Loading block %d / %x", block.Index, block.Hash)
	    db.blockIndexMap[block.Index] = BlockID(k)
            db.UpdateLatest(block.Hash)
	    return nil
	})
    })
}

func (db *BlockDB) GetBlockByIndex(blockIndex int) (*bc.Block, error) {
    blockID, ok := db.blockIndexMap[blockIndex]
    if !ok {
        return nil, xerrors.New("no block found")
    }
    block := db.GetByID(blockID)
    return block, nil
}

func (db *BlockDB) AppendRefererBlock(block *bc.Block) {
    db.refererMutex.Lock()
    defer db.refererMutex.Unlock()
    if len(db.refererBlocks) >= COSI_MEMBERS {
        db.refererBlocks = db.refererBlocks[len(db.refererBlocks)-COSI_MEMBERS+1:]
    }
    db.refererBlocks = append(db.refererBlocks, block.Copy())
}

func (db *BlockDB) GetLatestRefererBlock() *bc.Block {
    db.refererMutex.Lock()
    defer db.refererMutex.Unlock()
    if len(db.refererBlocks) == 0 {
        return nil
    }
    return db.refererBlocks[len(db.refererBlocks)-1]
}

func (db *BlockDB) GetRefererBlocks() []*bc.Block {
    db.refererMutex.Lock()
    defer db.refererMutex.Unlock()
    return db.refererBlocks
}

func (db *BlockDB) GetLeaderKey() string {
    block := db.GetLatestRefererBlock()
    if block != nil {
        return block.PublicKey
    }
    return ""
}

// blockBuffer will cache a referer block when the conode has
// verified it and it will later store it in the DB after the protocol
// has succeeded.
// Blocks are stored per skipchain to prevent the cache to grow and it
// is cleared after a new block is added
type blockBuffer struct {
        blocks []*bc.Block
	sync.Mutex
}

func newBlockBuffer() *blockBuffer {
    return &blockBuffer{}
}

func (bb *blockBuffer) Append(block *bc.Block) {
    bb.Lock()
    defer bb.Unlock()
    bb.blocks = append(bb.blocks, block.Copy())
}

func (bb *blockBuffer) Choice() *bc.Block {
    bb.Lock()
    defer func() {
	bb.blocks = bb.blocks[:0]
	bb.Unlock()
    }()
    if len(bb.blocks) == 0 {
        return nil
    }
    return bb.blocks[rand.Intn(len(bb.blocks))]
}
