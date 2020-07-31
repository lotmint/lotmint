package service

type Transaction struct {
    Version uint32
    Signature []byte
    PubKeyIn string
    PubKeyOut string
    Value uint64
    Timestamp uint64
    Nonce uint64
    // Store any data.
    Payload []byte
}

type txPool struct {
    sync.Mutex
    txsMap map[string][]Transaction
}

func newTxPool() txPool {
    return txPool{
        txsMap: make(map[string][]Transaction),
    }
}

func (p *txPool) add(key string, tx Transaction) {
    p.Lock()
    defer p.Unlock()

    if txs, ok := p.txsMap[key]; !ok {
	r.txsMap[key] = []Transaction{tx}
    } else {
	txs = append(txs, tx)
	p.txsMap[key] = txs
    }
}
