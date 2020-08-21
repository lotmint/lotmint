package service

import (
    "sync"

    bbolt "go.etcd.io/bbolt"
    bc "lotmint/blockchain"
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

type BlockDB struct {
    *bbolt.DB
    bucketName []byte
    latestBlockID BlockID
    latestMutex sync.Mutex
    blockIndexMap map[int]BlockID
}

// NewBlockDB returns an initialized BlockDB structure.
func NewBlockDB(db *bbolt.DB, bn[]byte) *BlockDB {
	return &BlockDB{
		DB: db,
		bucketName: bn,
		blockIndexMap: make(map[int]BlockID),
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
		return nil, xerrors.New("cannot look up skipblock with ID == nil")
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

	//return sbMsg.(*bc.Block).Copy(), nil
	return sbMsg.(*bc.Block), nil
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
	    block := msg.(*bc.Block)
	    //block := msg.(*bc.Block).Copy()
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

// blockBuffer will cache a proposed block when the conode has
// verified it and it will later store it in the DB after the protocol
// has succeeded.
// Blocks are stored per skipchain to prevent the cache to grow and it
// is cleared after a new block is added
type blockBuffer struct {
	buffer map[string]*bc.Block
	sync.Mutex
}

func newBlockBuffer() *blockBuffer {
    return &blockBuffer{
        buffer: make(map[string]*bc.Block),
    }
}
