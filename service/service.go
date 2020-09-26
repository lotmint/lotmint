/* Websocket */
package service

import (
	"bytes"
	"errors"
	"math/rand"
	// "fmt"
	"sync"
	"time"

	"lotmint"
        bc "lotmint/blockchain"
	"lotmint/blscosi"
	"lotmint/mining"
	"lotmint/protocol"
	"lotmint/utils"

        "go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
        "golang.org/x/xerrors"
)

var serviceID onet.ServiceID
//var peers []string

var (
    serviceMessageId network.MessageTypeID
    handshakeMessageId network.MessageTypeID
    blockMessageId network.MessageTypeID
    blockDownloadRequestId network.MessageTypeID
    blockDownloadResponseId network.MessageTypeID
    signatureRequestId network.MessageTypeID
    signatureResponseId network.MessageTypeID
)

var suite = pairing.NewSuiteBn256()

func init() {
    var err error
    serviceID, err = onet.RegisterNewService(lotmint.ServiceName, newService)
    log.ErrFatal(err)
    network.RegisterMessage(&storage{})
    serviceMessageId = network.RegisterMessage(&ServiceMessage{})
    handshakeMessageId = network.RegisterMessage(&HandshakeMessage{})
    blockMessageId = network.RegisterMessage(&BlockMessage{})
    blockDownloadRequestId = network.RegisterMessage(&DownloadBlockRequest{})
    blockDownloadResponseId = network.RegisterMessage(&DownloadBlockResponse{})
    signatureRequestId = network.RegisterMessage(&SignatureRequest{})
    signatureResponseId = network.RegisterMessage(&SignatureResponse{})
}

// Nonce is used to prevent replay attacks in instructions.
type Nonce [32]byte

// GenNonce returns a random nonce.
func GenNonce() (n Nonce) {
	random.Bytes(n[:], random.New())
	return n
}

func GenNonce64() uint64 {
    rand.Seed(int64(time.Now().Nanosecond()))
    return rand.Uint64()
}

type Service struct {
    // We need to embed the ServiceProcessor, so that incoming messages
    // are correctly handled.
    *onet.ServiceProcessor

    db *BlockDB

    blockBuffer *blockBuffer

    txPool txPool

    storage *storage

    peerStorage *peerStorage

    synChan chan RemoteServerIndex

    synDone bool

    createBlockChainMutex sync.Mutex

    startChan chan bool

    closeChan chan bool

    privateClock *PrivateClock

    preTimestamp uint64

    delta uint64

    timerRunning bool

    miner *mining.Miner

    proxyNodes []*network.ServerIdentity

    responseChan chan blscosi.ChannelResponse

    finalResponseChan chan blscosi.ChannelResponse
}

var storageID = []byte("main")

var peerStorageID = []byte("peers")

// storage is used to save our data.
type storage struct {
    Count int
    sync.Mutex
}

// Storage peer node
type peerStorage struct {
    sync.Mutex
    // Use map structure for peerNodes to quickly find duplicates
    peerNodeMap map[string]*network.ServerIdentity
}

// Clock starts a protocol and returns the run-time.
func (s *Service) Clock(req *lotmint.Clock) (*lotmint.ClockReply, error) {
    s.storage.Lock()
    s.storage.Count++
    s.storage.Unlock()
    s.save()
    tree := req.Roster.GenerateNaryTreeWithRoot(2, s.ServerIdentity())
    if tree == nil {
        return nil, errors.New("couldn't create tree")
    }
    log.Lvl3("Root children len:", len(tree.Root.Children))
    pi, err := s.CreateProtocol(protocol.Name, tree)
    if err != nil {
        return nil, err
    }
    start := time.Now()
    pi.Start()
    resp := &lotmint.ClockReply {
        Children: <-pi.(*protocol.LotMintProtocol).ChildCount,
    }
    resp.Time = time.Now().Sub(start).Seconds()
    return resp, nil
}

// Count returns the number of instantiations of the protocol.
func (s *Service) Count(req *lotmint.Count) (*lotmint.CountReply, error) {
    s.storage.Lock()
    defer s.storage.Unlock()
    return &lotmint.CountReply{Count: s.storage.Count}, nil
}

// Peer Operation
func (s *Service) Peer(req *lotmint.Peer) (*lotmint.PeerReply, error) {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
    var peers []*network.ServerIdentity
    var resp = &lotmint.PeerReply{}
    log.Lvl3("Peer command:", req.Command)
    switch req.Command {
    case "add":
	for _, peerNode := range req.PeerNodes{
	    if _, ok := s.peerStorage.peerNodeMap[peerNode.Public.String()]; !ok {
                peers = append(peers, peerNode)
	    }
	}
	if len(peers) > 0 {
	    go s.AddPeerServerIdentity(peers, true)
        }
    case "del":
	for _, peerNode := range req.PeerNodes{
	    if _, ok := s.peerStorage.peerNodeMap[peerNode.Public.String()]; ok {
                peers = append(peers, peerNode)
	    }
	}
	if len(peers) > 0 {
	    go s.RemovePeerServerIdentity(peers)
        }
    case "show":
	if len(req.PeerNodes) > 0 {
	    peers = req.PeerNodes
	} else {
	    for _, val := range s.peerStorage.peerNodeMap {
                peers = append(peers, val)
            }
	}
    default:
        return nil, xerrors.New("Command not supported")
    }
    resp.List = peers
    return resp, nil
}

// Broadcast message
func (s *Service) BroadcastBlock(block *bc.Block, type_ int) {
    for _, peer := range s.peerStorage.peerNodeMap {
        if err := s.SendRaw(peer, &BlockMessage{type_, block}); err != nil {
            log.Error(err)
        }
    }
}

// New BlockChain
func (s *Service) CreateGenesisBlock(req *lotmint.GenesisBlockRequest) (*bc.Block, error) {
    s.createBlockChainMutex.Lock()
    defer s.createBlockChainMutex.Unlock()
    if s.db.GetLatest() != nil {
        return nil, errors.New("You have already joined blockchain.")
    }
    genesisBlock := bc.GetGenesisBlock()
    genesisBlock.PublicKey = s.ServerIdentity().Public.String()
    go s.BroadcastBlock(genesisBlock, RefererBlock)
    // Add referer block into self refererBlocks
    s.db.AppendRefererBlock(genesisBlock)
    s.db.Store(genesisBlock)
    s.db.UpdateLatest(genesisBlock.Hash)
    go s.startTimer()
    go s.startMiner(genesisBlock)
    return genesisBlock, nil
}

// Leader start 
func (s *Service) startTimer() {
    if !s.isLeader() {
        return
    }
    if s.timerRunning {
        log.Lvl3("timer already started")
        return
    }
    s.timerRunning = true
    select {
    case <-time.After(60 * time.Second): // instead of 2Delta
        s.timerRunning = false
    }
    s.stopMiner()
    if !s.isLeader() {
        return
    }
    s.startChan <- true
}

func (s *Service) handleSignatureRequest(env *network.Envelope) error {
    req, ok := env.Msg.(*SignatureRequest)
    if !ok {
        return xerrors.Errorf("%v failed to cast to SignatureRequest", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    block := s.db.GetLatest()
    if block == nil {
        return xerrors.New("Blockchain not actived")
    }
    if block.PublicKey != env.ServerIdentity.Public.String() {
        return xerrors.New("No leader request")
    }
    var keyMap map[string]bool
    for _, b := range s.db.GetRefererBlocks() {
        keyMap[b.PublicKey] = true
    }
    var serverIdentities []*network.ServerIdentity
    for key, _ := range keyMap {
        if si, ok := s.peerStorage.peerNodeMap[key]; !ok {
            serverIdentities = append(serverIdentities, si)
        }
    }
    if len(serverIdentities) == 0 {
        return xerrors.New("latest mining nodes couldn't be empty")
    }

    // starstart
    go s.runProtocols(serverIdentities)

    res := <-s.responseChan

    // The hash is the message blscosi actually signs, we recompute it the
    // same way as blscosi and then return it.
    h := suite.Hash()
    h.Write(req.Message)

    return s.SendRaw(env.ServerIdentity, SignatureResponse{h.Sum(nil), res.Responses})
}

func (s *Service) runProtocols(serverIdentities []*network.ServerIdentity) {
    roster := onet.NewRoster(serverIdentities)
    tree := roster.GenerateNaryTreeWithRoot(len(serverIdentities) - 1, s.ServerIdentity())
    if tree == nil {
	log.Errorf("Couldn't create tree")
        return
    }
    log.Lvl3("Root children len:", len(tree.Root.Children))
    pi, err := s.CreateProtocol(blscosi.DefaultProtocolName, tree)
    if err != nil {
	log.Error(err)
        return
    }
    // start the protocol
    log.Lvl3("Cosi Service starting up root protocol")
    if err = pi.Start(); err != nil {
	log.Error(err)
        return
    }

    // wait for reply. This will always eventually return.
    res := <-pi.(*blscosi.BlsCosi).ResponseChan
    s.responseChan <- blscosi.ChannelResponse{res.Responses}
}

func (s *Service) handleSignatureResponse(env *network.Envelope) error {
    req, ok := env.Msg.(*SignatureResponse)
    if !ok {
        return xerrors.Errorf("%v failed to cast to SignatureResponse", s.ServerIdentity())
    }
    log.Lvl3("req:", req)

    s.finalResponseChan <- blscosi.ChannelResponse{req.Responses}

    return nil
}

func (s *Service) GetBlockByID(req *lotmint.BlockByIDRequest) (*bc.Block, error) {
    s.createBlockChainMutex.Lock()
    defer s.createBlockChainMutex.Unlock()
    block := s.db.GetLatest()
    if block == nil {
        return nil, xerrors.New("Blockchain not actived")
    }
    block = s.db.GetByID(BlockID(req.Value))
    if block == nil {
        return nil, xerrors.New("No such block")
    }
    return block, nil
}

func (s *Service) GetBlockByIndex(req *lotmint.BlockByIndexRequest) (*bc.Block, error) {
    s.createBlockChainMutex.Lock()
    defer s.createBlockChainMutex.Unlock()

    block := s.db.GetLatest()
    if block == nil {
        return nil, xerrors.New("Blockchain not actived")
    }
    if block.Index > req.Value {
        var err error
        block, err = s.db.GetBlockByIndex(req.Value)
	if err != nil {
            block = s.db.GetLatest()
            for block.Index >= 0 && block.Index != req.Value {
                block = s.db.GetByID(block.Hash)
                if block == nil {
                    return nil, errors.New("No such block")
                }
            }
	}
    }
    return block, nil

}

// NewProtocol is called on all nodes of a Tree (except the root, since it is
// the one starting the protocol) so it's the Service that will be called to
// generate the PI on all others node.
// If you use CreateProtocolOnet, this will not be called, as the Onet will
// instantiate the protocol on its own. If you need more control at the
// instantiation of the protocol, use CreateProtocolService, and you can
// give some extra-configuration to your protocol in here.
func (s *Service) NewProtocol(tn *onet.TreeNodeInstance, conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	log.Lvl3("Not templated yet")
	return nil, nil
}

// saves all data.
func (s *Service) save() {
    s.storage.Lock()
    defer s.storage.Unlock()
    err := s.Save(storageID, s.storage)
    if err != nil {
        log.Error("Couldn't save data:", err)
    }
    // Save peers data
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
    err = s.Save(peerStorageID, s.peerStorage)
    if err != nil {
        log.Error("Couldn't save peer data:", err)
    }
}

// Sync block with other peer node
func (s *Service) handshake() {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
    go s.processMessage(len(s.peerStorage.peerNodeMap))
    for _, peerNode := range s.peerStorage.peerNodeMap {
        err := s.SendRaw(peerNode, &HandshakeMessage{
	    GenesisID: s.db.GetGenesisID(),
	    LatestBlock: s.db.GetLatest(),
	    Answer: true,
	})
	if err != nil {
            log.Warn(err)
	}
    }
}

// Tries to load the configuration and updates the data in the service
// if it finds a valid config-file.
func (s *Service) tryLoad() error {
    s.storage = &storage{}
    msg, err := s.Load(storageID)
    if err != nil {
        return err
    }
    if msg == nil {
        return nil
    }
    var ok bool
    s.storage, ok = msg.(*storage)
    if !ok {
       return errors.New("Data of wrong type")
    }

    // Load peers data
    s.peerStorage = &peerStorage{}
    msg, err = s.Load(peerStorageID)
    if err != nil {
        return err
    }
    if msg == nil {
        return nil
    }
    s.peerStorage, ok = msg.(*peerStorage)
    if !ok {
       return errors.New("Peer data of wrong type")
    }
    s.handshake()

    return nil
}

func (s *Service) publicKey() string {
    return s.ServerIdentity().Public.String()
}

func (s *Service) isLeader() bool {
    leaderKey := s.db.GetLeaderKey()
    if leaderKey == "" {
        return false
    }
    return leaderKey == s.publicKey()
}

func (s *Service) AddPeerServerIdentity(peers []*network.ServerIdentity, needConn bool) {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
    for _, peer := range peers {
	// Self ServerIdentity
        // s.ServerIdentity()
	if needConn {
	    err := s.SendRaw(peer, &ServiceMessage{"Test Service Message"})
	    if err != nil {
                log.Error(err)
                continue
	    }
        }
	s.peerStorage.peerNodeMap[peer.Public.String()] = peer
    }
}

func (s *Service) RemovePeerServerIdentity(peers []*network.ServerIdentity) {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
    for _, peer := range peers {
        if _, ok := s.peerStorage.peerNodeMap[peer.Public.String()]; ok {
	    delete(s.peerStorage.peerNodeMap, peer.Public.String())
        }
    }
}

func (s *Service) processMessage(n int) {
    if n == 0 {
        return
    }
    var remotes []RemoteServerIndex
    wg := sync.WaitGroup{}
    wg.Add(1)
    go func() {
        defer wg.Done()
        count := 0
        for count < n {
            select {
	    case remote := <-s.synChan:
                latestBlock := s.db.GetLatest()
	        if latestBlock == nil || remote.Index > latestBlock.Index {
		    remotes = append(remotes, remote)
	        }
	        count++
            case <-time.After(5 * time.Second):
	        return
	    }
        }
    }()
    wg.Wait()
    if len(remotes) == 0 {
        s.synDone = true
    } else {
	for len(remotes) > 0 {
            start := 0
	    randIndex := rand.Intn(len(remotes))
            remote := remotes[randIndex]
            latestBlock := s.db.GetLatest()
            if latestBlock != nil {
                start = latestBlock.Index
            }
            size := MAX_BLOCK_PERONCE
            if remote.Index < size {
                size = remote.Index
            }
            err := s.SendRaw(remote.ServerIdentity, &DownloadBlockRequest{
	        GenesisID: s.db.GetGenesisID(),
	        Start: start,
	        Size: size,
            })
	    if err == nil {
                break
	    }
            remotes = append(remotes[:randIndex], remotes[randIndex+1:]...)
	}
	if len(remotes) == 0 {
            log.Error("failed to connect all remote servers, retrying")
            s.handshake()
	}
    }
}

func (s *Service) runCosiProtocol() {
    // TODO: COSI proposal
    err := s.SendRaw(s.proxyNodes[0], &SignatureRequest{
	Message: []byte("Test Client"),
    });
    if err != nil {
        log.Error("Couldn't connect proxy server:", err)
	return
    }
    select {
    case res := <-s.finalResponseChan:
        // TODO: Verify message and append responses to txBuffer
	log.Infof("Cosi responses: %v", res.Responses)
    case <-time.After(5 * time.Second):
        log.Lvl3("timer already started")
    }
}

func (s *Service) createTxBlock() error {
    s.createBlockChainMutex.Lock()
    defer s.createBlockChainMutex.Unlock()
    latest := s.db.GetLatest()
    if latest == nil {
        return xerrors.New("invalid blockchain")
    }
    bh := latest.BlockHeader.Copy()
    bh.Index = bh.Index + 1
    bh.Nonce = GenNonce64()
    bh.Timestamp = getCurrentTimestamp()
    copy(bh.PrevBlock, latest.Hash)
    newBlock := &bc.Block{
        BlockHeader: bh,
    }
    hash, err := newBlock.CalculateHash()
    if err != nil {
        return err
    }
    newBlock.Hash = hash
    newBlock.PublicKey = s.ServerIdentity().Public.String()
    s.BroadcastBlock(newBlock, TxBlock)
    s.db.Store(newBlock)
    s.db.UpdateLatest(hash)

    go s.runCosiProtocol()

    return nil
}

func (s *Service) startTxBlockGenerator() {
    count := 0
    timer := time.NewTimer(10 * time.Duration(s.delta))
    for {
        select {
	case <-timer.C:
	    if err := s.createTxBlock(); err != nil {
		log.Errorf("failed to create tx block:", err)
                return
	    }
	    count++
	    if count >= EPOCH {
	        // s.closeChan <- true
                log.Lvl3("epoch finished")
		// Broadcast RefererBlock and starting mining
		block := s.blockBuffer.Choice()
                go s.BroadcastBlock(block, RefererBlock)
                // Add referer block into self refererBlocks
	        s.db.AppendRefererBlock(block)
		// Store RefererBlock into local blockchain
                s.db.Store(block)
                s.db.UpdateLatest(block.Hash)
                go s.startTimer()
                go s.startMiner(block)
	        return
	    }
	    timer.Reset(10 * time.Duration(s.delta))
	case <-s.closeChan:
            log.Lvl3("tx block generator stopped")
	    return
	}
    }
}

func (s *Service) mainLoop() {
    for {
        select {
	case <-s.startChan:
	    go s.startTxBlockGenerator()
	}
    }
}

// handleHandshakeMessage
func (s *Service) handleHandshakeMessage(env *network.Envelope) error {
    req, ok := env.Msg.(*HandshakeMessage)
    if !ok {
        return xerrors.New("error while unmarshaling a message")
    }
    genesisID := s.db.GetGenesisID()
    if genesisID == nil {
        if req.GenesisID == nil || req.LatestBlock == nil {
	    log.Warn("neither local nor remote chain started")
            return nil
        }
    } else {
	if req.GenesisID != nil && bytes.Compare(req.GenesisID, genesisID) != 0 {
            return xerrors.New("no same block chain")
        }
        s.db.latestMutex.Lock()
        latestBlock := s.db.GetLatest()
        s.db.latestMutex.Unlock()
	if req.LatestBlock != nil && latestBlock.Index > req.LatestBlock.Index {
	    log.Warn("the local block height ahead of the remote block(skip)")
            return nil
        }
        if req.Answer && (req.GenesisID == nil || latestBlock.Index > req.LatestBlock.Index) {
	    err := s.SendRaw(env.ServerIdentity, &HandshakeMessage{
	        GenesisID: s.db.GetGenesisID(),
	        LatestBlock: latestBlock,
	        Answer: false,
	    })
	    if err != nil {
	        log.Warn(err)
	    }
        }
	if (req.GenesisID == nil || req.LatestBlock == nil) {
	    log.Warn("the remote block is empty(skip)")
            return nil
	}
    }
    s.synChan <- RemoteServerIndex{
        Index: req.LatestBlock.Index,
	ServerIdentity: env.ServerIdentity,
    }
    return nil
}

// handleMessageReq messages.
func (s *Service) handleMessageReq(env *network.Envelope) error {
    // Parse message.
    req, ok := env.Msg.(*ServiceMessage)
    if !ok {
        return xerrors.Errorf("%v failed to cast to MessageReq", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    s.AddPeerServerIdentity([]*network.ServerIdentity{env.ServerIdentity}, false)
    return nil
}

// handleBlockMessage
func (s *Service) handleBlockMessage(env *network.Envelope) error {
    req, ok := env.Msg.(*BlockMessage)
    if !ok {
        return xerrors.Errorf("%v failed to cast to BlockMessage", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    // TODO: Check block validation
    // Drop block if it is greater than delta+phi
    if s.preTimestamp > 0 && s.delta > 0 && req.Block.Timestamp > s.preTimestamp + s.delta + PHI {
        log.Warnf("block timeout: %v", req.Block)
        return xerrors.Errorf("block timeout: %v", req.Block)
    }
    block := req.Block
    switch req.Type {
    case CandidateBlock:
	s.processCandidateBlock(block)
    case RefererBlock:
	b, err := s.db.GetBlockByIndex(block.Index)
	if b != nil && err == nil {
            /*if req.Block.Index == 0 && bytes.Compare(b.Hash, req.Block.Hash) == 0 {
                // Add referer block into refererBlocks
	        s.db.AppendRefererBlock(block)
	    }*/
            log.Infof("Block '%d' already exists", b.Index)
            return nil
	}
	// reset delta every block request
	/*delta, err := s.calcDelta()
	if err == nil {
	    s.delta = delta
        } else {
            log.Warn(err)
	}*/
	s.preTimestamp = getCurrentTimestamp()

        go s.BroadcastBlock(block, req.Type)
        // Add referer block into refererBlocks
	s.db.AppendRefererBlock(block)
        s.db.Store(block)
	// Propogate continue
        s.db.UpdateLatest(block.Hash)
        go s.startTimer()
        go s.startMiner(block)
    case TxBlock:
        s.db.Store(block)
        s.BroadcastBlock(block, req.Type)
        s.db.UpdateLatest(block.Hash)
    default:
        return xerrors.Errorf("type '%d' not handled", req.Type)
    }
    return nil
}

// downloadBlockRequest
func (s *Service) handleDownloadBlockRequest(env *network.Envelope) error {
    req, ok := env.Msg.(*DownloadBlockRequest)
    if !ok {
        return xerrors.Errorf("%v failed to cast to DownloadBlockRequest", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    genesisID := s.db.GetGenesisID()
    if bytes.Compare(req.GenesisID, genesisID) != 0 {
	log.Error("no genesis block found")
        return nil
    }
    var blocks []*bc.Block
    for index := req.Start; index < req.Size; index++ {
	block, err := s.db.GetBlockByIndex(index)
	if err != nil {
            break
	}
        blocks = append(blocks, block.Copy())
    }
    if len(blocks) == 0 {
        return xerrors.New("no valid blocks")
    }
    return s.SendRaw(env.ServerIdentity, &DownloadBlockResponse{
        Blocks: blocks,
        GenesisID: genesisID,
    })
}

// downloadBlockResponse
func (s *Service) handleDownloadBlockResponse(env *network.Envelope) error {
    req, ok := env.Msg.(*DownloadBlockResponse)
    if !ok {
        return xerrors.Errorf("%v failed to cast to DownloadBlockResponse", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    genesisID := s.db.GetGenesisID()
    if bytes.Compare(req.GenesisID, genesisID) != 0 {
	log.Error("no genesis block found")
        return nil
    }
    index := -1
    for _, block := range req.Blocks {
	block, _ := s.db.GetBlockByIndex(block.Index)
	if block != nil {
            log.Warnf("block index %d already exists, it will be override", block.Index)
	}
	if block.Index > index {
            index = block.Index
	}
        s.db.Store(block)
    }
    if index > 0 {
        s.db.UpdateLatest(req.Blocks[index].Hash)
    }
    s.handshake()
    return nil
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
    db, bucket := c.GetAdditionalBucket([]byte("blockdb"))
    s := &Service{
        ServiceProcessor: onet.NewServiceProcessor(c),
	db: NewBlockDB(db, bucket),
	blockBuffer: newBlockBuffer(),
	txPool: newTxPool(),
	peerStorage: &peerStorage{
	    peerNodeMap: make(map[string]*network.ServerIdentity),
	},
	synChan: make(chan RemoteServerIndex, 1),
	synDone: false,
	startChan: make(chan bool, 1),
	closeChan: make(chan bool, 1),
	privateClock: newPrivateClock(COSI_MEMBERS),
	delta: DEFAULT_DELTA,
	timerRunning: false,
	responseChan: make(chan blscosi.ChannelResponse),
	finalResponseChan: make(chan blscosi.ChannelResponse),
    }
    s.miner = mining.New(s.minerCallback)
    defaultProxy, err := utils.ConvertPeerURL(DEFAULT_PROXY)
    if err != nil {
        log.Error(err)
	return nil, err
    }
    s.proxyNodes = []*network.ServerIdentity{
        defaultProxy,
    }
    if err := s.db.BuildIndex(); err != nil {
        log.Error(err)
	return nil, err
    }
    if err := s.RegisterHandlers(s.Clock, s.Count); err != nil {
        return nil, errors.New("Couldn't register handlers")
    }
    if err := s.RegisterHandlers(s.Peer, s.CreateGenesisBlock, s.GetBlockByID, s.GetBlockByIndex); err != nil {
        return nil, errors.New("Couldn't register handlers")
    }
    // s.ServiceProcessor.RegisterStatusReporter("BlockDB", s.db)
    s.RegisterProcessorFunc(handshakeMessageId, s.handleHandshakeMessage)
    s.RegisterProcessorFunc(serviceMessageId, s.handleMessageReq)
    s.RegisterProcessorFunc(blockMessageId, s.handleBlockMessage)
    s.RegisterProcessorFunc(blockDownloadRequestId, s.handleDownloadBlockRequest)
    s.RegisterProcessorFunc(blockDownloadResponseId, s.handleDownloadBlockResponse)
    s.RegisterProcessorFunc(signatureRequestId, s.handleSignatureRequest)
    s.RegisterProcessorFunc(signatureResponseId, s.handleSignatureResponse)
    if err := s.tryLoad(); err != nil {
        log.Error(err)
	return nil, err
    }
    go s.mainLoop()
    return s, nil
}

func (s *Service) startMiner(block *bc.Block) {
    s.miner.Start(block)
}

func (s *Service) stopMiner() {
    s.miner.Stop()
}

func (s *Service) minerCallback(block *bc.Block) {
    block.PublicKey = s.ServerIdentity().Public.String()
    // TODO: abort mined block if after 2Delta
    s.processCandidateBlock(block)
}

func (s *Service) processCandidateBlock(block *bc.Block) {
    if s.isLeader() && s.timerRunning {
        // Add block into blockBuffer
        s.blockBuffer.Append(block)
    } else {
        s.BroadcastBlock(block, CandidateBlock)
    }
}
