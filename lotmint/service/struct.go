
package service

// HashID represents the Hash of the Block
type HashID []byte

// Announce is used to pass a message to all children.
type ServiceMessage struct {
    Message string
}

type BlockDB struct {
    *bbolt.DB
    bucketName []byte
}

// NewBlockDB returns an initialized BlockDB structure.
func NewBlockDB(db *bbolt.DB, bn[]byte) *BlockDB {
	return &BlockDB{
		DB: db,
		bucketName: bn,
	}
}

// storeToTx stores the block into the database.
func (db *BlockDB) storeToTx(tx *bbolt.Tx, sb *Block) error {
	key := sb.Hash
	val, err := network.Marshal(sb)
	if err != nil {
		return err
	}
	return tx.Bucket([]byte(db.bucketName)).Put(key, val)
}

// getFromTx returns the skipblock identified by sbID.
// nil is returned if the key does not exist.
func (db *BlockDB) getFromTx(tx *bbolt.Tx, sbID BlockID) (*Block, error) {
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

	return sbMsg.(*Block).Copy(), nil
}

// Stores the set of blocks in the boltdb
func (db *BlockDB) StoreBlocks(blocks []*Block) ([]HashID, error) {
    var result []HashID
    err := db.Update(func(tx *bbolt.Tx) error {
        for i, block := range blocks {
           log.Lvlf2("Storing skipblock %d / %x", block.Index, block.Hash)
	   err := db.storeToTx(tx, block)
	   if err != nil {
	        return err
	   }
	   result = append(result, block.Hash)
	}
        return nil
    })
    return result, err
}

// Stores the given Block into db
func (db *BlockDB) Store(block *Block) HashID {
	ids, err := db.StoreBlocks([]*Block{block})
	if err != nil {
		log.Error(err)
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return nil
}

// blockBuffer will cache a proposed block when the conode has
// verified it and it will later store it in the DB after the protocol
// has succeeded.
// Blocks are stored per skipchain to prevent the cache to grow and it
// is cleared after a new block is added
type blockBuffer struct {
	buffer map[string]*Block
	sync.Mutex
}

func newBlockBuffer() *blockBuffer {
	return &blockBuffer{
		buffer: make(map[string]*Block),
	}
}
