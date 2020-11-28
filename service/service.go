/* Websocket */
package service

import (
	"bytes"
	"errors"
	"math/rand"
    "net"
	// "fmt"
	"sync"
	"time"

	"lotmint"
    bc "lotmint/blockchain"
	"lotmint/mining"
	"lotmint/protocol"


    "go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
	"go.dedis.ch/protobuf"
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

    proxyRequestId network.MessageTypeID
    proxyResponseId network.MessageTypeID

    addressMessageId network.MessageTypeID
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

    proxyRequestId = network.RegisterMessage(&ProxyRequest{})
    proxyResponseId = network.RegisterMessage(&ProxyResponse{})

    addressMessageId = network.RegisterMessage(&AddressMessage{})
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

    proxyStorage *peerStorage

    synChan chan RemoteServerIndex

    synDone bool

    createBlockChainMutex sync.Mutex

    proposalChan chan bool

    closeChan chan bool

    privateClock *PrivateClock

    preTimestamp uint64

    delta uint64

    timerRunning bool

    miner *mining.Miner

    udpConn *net.UDPConn

    proxyResponseChan chan ProxyResponse

    signatureResponseChan chan SignatureResponse
}

var storageID = []byte("main")

var peerStorageID = []byte("peers")

var proxyStorageID = []byte("proxies")

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

// Proxy Operation
func (s *Service) Proxy(req *lotmint.Proxy) (*lotmint.PeerReply, error) {
    s.proxyStorage.Lock()
    defer s.proxyStorage.Unlock()
    var peers []*network.ServerIdentity
    var resp = &lotmint.PeerReply{}
    log.Lvl3("Proxy command:", req.Command)
    switch req.Command {
    case "add":
	    for _, peerNode := range req.PeerNodes{
            if peerNode.Public == nil {
                peerNode.Public = s.ServerIdentity().Public
            }
	        if _, ok := s.proxyStorage.peerNodeMap[peerNode.String()]; !ok {
	            s.proxyStorage.peerNodeMap[peerNode.String()] = peerNode
                peers = append(peers, peerNode)
                go s.BroadcastAddressTx(&AddressMessage{
                    Address: peerNode,
                })
	        }
	    }
        if len(peers) > 0 {
        }
    case "del":
	    for _, peerNode := range req.PeerNodes{
	        if _, ok := s.proxyStorage.peerNodeMap[peerNode.String()]; ok {
	            delete(s.proxyStorage.peerNodeMap, peerNode.String())
                peers = append(peers, peerNode)
	        }
	    }
    case "show":
	    if len(req.PeerNodes) > 0 {
	        peers = req.PeerNodes
	    } else {
	        for _, val := range s.proxyStorage.peerNodeMap {
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

// Broadcast block to all BFT members
func (s *Service) broadcastBlockToBFT(block *bc.Block) {
    latestBlock := s.db.GetLatest()
    if latestBlock == nil {
        log.Error("Blockchain not actived")
        return
    }
    tmpBlock := latestBlock.Copy()

    var memberMap map[string]bool
    for i := 0; i < COSI_MEMBERS; i++ {
        memberMap[tmpBlock.PublicKey] = true
        if tmpBlock.PrevBlock == nil {
            break
        }
        tmpBlock := s.db.GetByID(BlockID(tmpBlock.PrevBlock))
        if tmpBlock == nil {
            log.Error("Incomplete local blockchain")
            break
        }
    }

    for key, _ := range memberMap {
        if key == s.publicKey() {
            s.blockBuffer.Append(block)
        } else if peer, ok := s.peerStorage.peerNodeMap[key]; ok {
            if err := s.SendRaw(peer, &BlockMessage{CandidateBlock, block}); err != nil {
                log.Error(err)
            }
        }
    }

    if len(memberMap) == 0 {
        log.Error("BFT members is empty")
    }
}

func (s *Service) BroadcastBlockToProxies(block *bc.Block) {
    latest := s.db.GetLatest()
    if latest == nil {
        log.Errorf("Leader not found")
        return
    }
    for _, addr := range latest.Addresses {
        // Instead of udp protocol
	    /*if peer, ok := s.proxyStorage.peerNodeMap[addr.String()]; ok {
            if err := s.SendRaw(peer, &BlockMessage{ProxyCandidateBlock, block}); err != nil {
                log.Error(err)
            }
        }*/
        address := addr.Address.Host() + ":" + addr.Address.Port()
        log.Lvlf3("Connecting %s", address)
        conn, err := net.Dial(UDP, address)
        if err != nil {
            log.Error(err)
            conn.Close()
            continue
        }
        buf, err := protobuf.Encode(block)
        if err != nil {
            log.Error(err)
            conn.Close()
            continue
        }
        conn.Write(buf)
        // conn.Read(buf)
        conn.Close()
    }
}

func (s *Service) BroadcastAddressTx(addrTx *AddressMessage) {
    for _, peer := range s.peerStorage.peerNodeMap {
        if err := s.SendRaw(peer, addrTx); err != nil {
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

    // Store and broadcast block
    s.db.Store(genesisBlock)
    s.db.UpdateLatest(genesisBlock)
    s.BroadcastBlock(genesisBlock, RefererBlock)

    // Start local mining
    go s.startTimer(genesisBlock)

    return genesisBlock, nil
}

// Leader proposal at waiting for a while
func (s *Service) startTimer(block *bc.Block) {
    go s.startMiner(block)
    if !s.isLeader() {
        return
    }
    if s.timerRunning {
        log.Lvl3("timer already started")
        return
    }
    s.timerRunning = true
    select {
    // instead of 2Delta
    case <-time.After(60 * time.Second):
        s.timerRunning = false
    }
    log.Lvlf3("timer finished (%v)", time.Now().Format("2020-12-30 00:00:00"))

    // Notify stop local mining
    s.stopMiner()

    if !s.isLeader() {
        return
    }

    s.proposalChan <- true
}

func (s *Service) handleSignatureRequest(env *network.Envelope) error {
    req, ok := env.Msg.(*SignatureRequest)
    if !ok {
        return xerrors.Errorf("%v failed to cast to SignatureRequest", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    latest := s.db.GetLatest()
    if latest == nil {
        return xerrors.New("Blockchain not actived")
    }

    var blockMap map[string]*bc.Block
    for _, ob := range req.Block.OrderBlocks {
        blockMap[ob.PublicKey] = ob
    }

    var omitBlocks map[string]*bc.Block
    blocks := s.blockBuffer.Choice()
    for _, block := range blocks {
        if b, ok := blockMap[block.PublicKey]; !ok {
            omitBlocks[b.PublicKey] = b
        }
    }

    return s.SendRaw(env.ServerIdentity, SignatureResponse{s.publicKey(), omitBlocks})
}

func (s *Service) handleSignatureResponse(env *network.Envelope) error {
    req, ok := env.Msg.(*SignatureResponse)
    if !ok {
        return xerrors.Errorf("%v failed to cast to SignatureResponse", s.ServerIdentity())
    }
    log.Lvl3("req:", req)

    s.signatureResponseChan <- SignatureResponse{env.ServerIdentity.Public.String(), req.BlockMap}

    return nil
}

func (s *Service) handleProxyRequest(env *network.Envelope) error {
    req, ok := env.Msg.(*ProxyRequest)
    if !ok {
        return xerrors.Errorf("%v failed to cast to ProxyRequest", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    if req.Block == nil {
        return xerrors.New("ProxyRequest block is nil")
    }
    latestBlock := s.db.GetLatest()
    if latestBlock == nil {
        return xerrors.New("Blockchain not actived")
    }

    tmpBlock := latestBlock.Copy()

    var memberMap map[string]bool
    for i := 0; i < COSI_MEMBERS; i++ {
        memberMap[tmpBlock.PublicKey] = true
        if tmpBlock.PrevBlock == nil {
            break
        }
        tmpBlock := s.db.GetByID(BlockID(tmpBlock.PrevBlock))
        if tmpBlock == nil {
            return xerrors.New("Incomplete local blockchain")
        }
    }

    success := 0
    wg := sync.WaitGroup{}
    var errs []error
    for key, _ := range memberMap {
        if peer, ok := s.peerStorage.peerNodeMap[key]; ok {
            success++
            name := peer.Address.String()
            wg.Add(1)
            go func() {
               if err := s.SendRaw(peer, &SignatureRequest{req.Block}); err != nil {
                   errs = append(errs, xerrors.Errorf("%s: %v", name, err))
               }
               wg.Done()
            }()
        }
    }
    wg.Wait()

    if len(errs) > 0 {
        log.Error(errs)
    }

    var responses map[string]map[string]*bc.Block
    done := len(errs)
    for done < success {
        select {
        case reply := <-s.signatureResponseChan:
            responses[reply.PublicKey] = reply.BlockMap
            done++
        case <-time.After(5):
            log.Lvlf3("timeout waiting for all followers response: %v", s.ServerIdentity())
			done = success
        }
    }

    // TODO: Count followers blocks and fill into OrderBlocks
    var blockCountMap map[string]int
    var blockMap map[string]*bc.Block
    for _, resp := range responses {
        for key, res := range resp {
            if _, ok := blockCountMap[key]; !ok {
                blockCountMap[key] = 0
                blockMap[key] = res
            }
            blockCountMap[key] += 1
        }
    }

    var omitBlocks map[string]*bc.Block
    for key, val := range blockCountMap {
        block, found := blockMap[key]
        if val >= 2 * len(responses) / 3 && found {
            omitBlocks[key] = block
        }
    }

    // ProxyResponse Message
    return s.SendRaw(env.ServerIdentity, ProxyResponse{env.ServerIdentity.Public.String(), omitBlocks})
}

func (s *Service) handleProxyResponse(env *network.Envelope) error {
    req, ok := env.Msg.(*SignatureResponse)
    if !ok {
        return xerrors.Errorf("%v failed to cast to SignatureRequest", s.ServerIdentity())
    }
    log.Lvl3("req:", req)

    s.proxyResponseChan <- ProxyResponse{env.ServerIdentity.Public.String(), req.BlockMap}

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

    // Save proxies data
    s.proxyStorage.Lock()
    defer s.proxyStorage.Unlock()
    err = s.Save(proxyStorageID, s.proxyStorage)
    if err != nil {
        log.Error("Couldn't save proxy data:", err)
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
    msg, err := s.Load(storageID)
    if err != nil {
        return err
    }
    var ok bool
    if msg != nil {
        //s.storage = &storage{}
        s.storage, ok = msg.(*storage)
        if !ok {
           return errors.New("Data of wrong type")
        }
    }

    // Load peers data
    msg, err = s.Load(peerStorageID)
    if err != nil {
        return err
    }
    if msg != nil {
        //s.peerStorage = &peerStorage{}
        s.peerStorage, ok = msg.(*peerStorage)
        if !ok {
           return errors.New("Peer data of wrong type")
        }
    } else {
    }

    // Load proxies data
    msg, err = s.Load(proxyStorageID)
    if err != nil {
        return err
    }
    if msg != nil {
        //s.proxyStorage = &peerStorage{}
        s.proxyStorage, ok = msg.(*peerStorage)
        if !ok {
           return errors.New("Proxy data of wrong type")
        }
    }

    s.handshake()

    return nil
}

func (s *Service) publicKey() string {
    return s.ServerIdentity().Public.String()
}

func (s *Service) isLeader() bool {
    latest := s.db.GetLatest()
    if latest != nil {
        // log.Lvlf3("BlockchanKey: %s, PublicKey: %s", latest.PublicKey, s.publicKey())
        return latest.PublicKey == s.publicKey()
    }
    return false
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

func (s *Service) runProposal() {
    log.Info("Leader starting to refer block")
    latest := s.db.GetLatest()
    if latest == nil {
        log.Error("Blockchain not actived")
        return
    }
    if len(latest.Addresses) == 0 {
        log.Error("No gateway actived")
        return
    }
    blocks := s.blockBuffer.Choice()
    if len(blocks) == 0 {
        // TODO: Retransmission Message
        log.Warn("No candidate blocks: reset")
        return
    }
    var blockKeyMap map[string]bool
    for _, b := range blocks {
        blockKeyMap[b.PublicKey] = true
    }
    var errs []error
    success := 0
    wg := sync.WaitGroup{}
    proposalBlock := blocks[0]
    proposalBlock.OrderBlocks = blocks[1:]
    for _, addr := range latest.Addresses {
        if peer, ok := s.peerStorage.peerNodeMap[addr.Public.String()]; ok {
            success++
            name := peer.Address.String()
            wg.Add(1)
            go func() {
               if err := s.SendRaw(peer, &ProxyRequest{proposalBlock}); err != nil {
                   errs = append(errs, xerrors.Errorf("%s: %v", name, err))
               }
               wg.Done()
            }()
        }
    }

    wg.Wait()

    if len(errs) > 0 {
        log.Error(errs)
    }

    var responses map[string]map[string]*bc.Block
    done := len(errs)
    for done < success {
        select {
        case reply := <-s.proxyResponseChan:
            responses[reply.PublicKey] = reply.BlockMap
            done++
        case <-time.After(5):
            log.Lvlf3("timeout waiting for all proxy nodes responses: %v", s.ServerIdentity())
			done = success
        }
    }

    // TODO: Count followers blocks and fill into OrderBlocks
    var blockCountMap map[string]int
    var blockMap map[string]*bc.Block
    for _, resp := range responses {
        for key, res := range resp {
            if _, ok := blockCountMap[key]; !ok {
                blockCountMap[key] = 0
                blockMap[key] = res
            }
            blockCountMap[key] += 1
        }
    }
    for key, val := range blockCountMap {
        _, found := blockKeyMap[key]
        if val >= 2 * len(responses) / 3 && !found {
            proposalBlock.OrderBlocks = append(proposalBlock.OrderBlocks, blockMap[key])
        }
    }
    // Store and broadcast block
    s.db.Store(proposalBlock)
    s.db.UpdateLatest(proposalBlock)
    s.BroadcastBlock(proposalBlock, RefererBlock)
}

func (s *Service) mainLoop() {
    for {
        select {
	    case <-s.proposalChan:
	        go s.runProposal()
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

// handleAddressMessage
func (s *Service) handleAddressMessage(env *network.Envelope) error {
    req, ok := env.Msg.(*AddressMessage)
    if !ok {
        return xerrors.Errorf("%v failed to cast to AddressMessage", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    if req.Address != nil {
	    if _, ok := s.proxyStorage.peerNodeMap[req.Address.String()]; !ok {
	        s.proxyStorage.peerNodeMap[req.Address.String()] = req.Address
        }
        s.BroadcastAddressTx(&AddressMessage{
            Address: req.Address,
        })
    }
    return nil
}

// handleBlockMessage
func (s *Service) handleBlockMessage(env *network.Envelope) error {
    req, ok := env.Msg.(*BlockMessage)
    if !ok {
        return xerrors.Errorf("%v failed to cast to BlockMessage", s.ServerIdentity())
    }
    log.Lvl3("req:", req)
    block := req.Block
    if block == nil {
        return xerrors.New("Block could no be empty")
    }
    // TODO: Check block validation
    // Drop block if it is greater than delta+phi
    if s.db.GetLatest() != nil && block.Index > 0 {
        if s.preTimestamp > 0 && s.delta > 0 && req.Block.Timestamp > s.preTimestamp + s.delta + PHI {
            log.Warnf("block timeout: %v", req.Block)
            return xerrors.Errorf("block timeout: %v", req.Block)
        }
    }
    switch req.Type {
        // Deprecate: instead of udp
        /*case ProxyCandidateBlock:
	        peer, ok := s.peerStorage.peerNodeMap[block.PublicKey]
            if !ok {
                err := xerrors.New("leader disconnected")
                log.Error(err)
                return err
            }
            if err := s.SendRaw(peer, &BlockMessage{CandidateBlock, block}); err != nil {
                log.Error(err)
                return err
            }*/
        case CandidateBlock:
            // Add block into blockBuffer
            s.blockBuffer.Append(block)
        case RefererBlock:
            _block := s.db.GetByID(block.Hash)
            if _block != nil {
                log.Infof("Block '%d' already exists", block.Hash)
                return nil
            }
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

            // Add referer block into refererBlocks
	        // s.db.AppendRefererBlock(block)
            s.db.Store(block)
            s.db.UpdateLatest(block)
	        // Propogate continue
            go s.BroadcastBlock(block, req.Type)
            go s.startTimer(block)
    case TxBlock:
        s.db.Store(block)
        s.BroadcastBlock(block, req.Type)
        s.db.UpdateLatest(block)
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
        s.db.UpdateLatest(req.Blocks[index])
    }
    s.handshake()
    return nil
}

func (s *Service) handleUDPRequest() {
    for {
        buf := make([]byte, UDP_BUFFER_SIZE)
        l, cAddr, err := s.udpConn.ReadFromUDP(buf)
        if err != nil {
            log.Error(err)
            continue
        }
        log.Infof("Receive data[%d] from %s", l, cAddr)
        block := bc.NewBlock()
        //err = protobuf.DecodeWithConstructors(buf, block, network.DefaultConstructors(suite))
        err = protobuf.DecodeWithConstructors(buf, block, nil)
		if err != nil {
            log.Error(err)
            continue
		}

        /*s.udpConn.WriteToUDP([]byte(), cAddr)
        if err != nil {
            log.Error(err)
            return
        }*/
        // TODO: Broadcast to all BFT members
        go s.broadcastBlockToBFT(block)
    }
    defer s.udpConn.Close()
}

func (s *Service) startUDPServer() error {
    // Starting UDP Server for client puzzle
    address := ":" + s.ServerIdentity().Address.Port()
    log.Infof("Starting udp server on address udp://0.0.0.0%s", address)
    udpAddr, _ := net.ResolveUDPAddr(UDP, address)
    udpConn, err := net.ListenUDP(UDP, udpAddr)
    if err != nil {
        log.Error(err)
        return err
    }

    s.udpConn = udpConn
    go s.handleUDPRequest()

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
	    proxyStorage: &peerStorage{
            peerNodeMap: make(map[string]*network.ServerIdentity),
        },
	    synChan: make(chan RemoteServerIndex, 1),
	    synDone: false,
	    proposalChan: make(chan bool, 1),
	    closeChan: make(chan bool, 1),
	    privateClock: newPrivateClock(COSI_MEMBERS),
	    delta: DEFAULT_DELTA,
	    timerRunning: false,
	    proxyResponseChan: make(chan ProxyResponse),
	    signatureResponseChan: make(chan SignatureResponse),
    }
    if err := s.startUDPServer(); err != nil {
        log.Error(err)
        return nil, err
    }
    s.miner = mining.New(s.minerCallback)
    if err := s.db.BuildIndex(); err != nil {
        log.Error(err)
        return nil, err
    }
    if err := s.RegisterHandlers(s.Clock, s.Count); err != nil {
        return nil, errors.New("Couldn't register handlers")
    }
    if err := s.RegisterHandlers(s.Peer, s.Proxy, s.CreateGenesisBlock, s.GetBlockByID, s.GetBlockByIndex); err != nil {
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

    s.RegisterProcessorFunc(proxyRequestId,  s.handleProxyRequest)
    s.RegisterProcessorFunc(proxyResponseId, s.handleProxyResponse)

    s.RegisterProcessorFunc(addressMessageId, s.handleAddressMessage)

    if err := s.tryLoad(); err != nil {
        log.Error(err)
	    return nil, err
    }
    go s.mainLoop()
    return s, nil
}

func (s *Service) startMiner(block *bc.Block) {
    templateBlock := block.Copy()
    var addrKey []string
    for key, _ := range s.proxyStorage.peerNodeMap {
        addrKey = append(addrKey, key)
    }
    n := COSI_MEMBERS
    for n > 0 && len(addrKey) > 0 {
        index := rand.Intn(len(addrKey))
        templateBlock.Addresses = append(templateBlock.Addresses, s.proxyStorage.peerNodeMap[addrKey[index]])
        addrKey = append(addrKey[:index], addrKey[index+1:]...)
        n--
    }
    s.miner.Start(templateBlock)
}

func (s *Service) stopMiner() {
    s.miner.Stop()
}

func (s *Service) minerCallback(block *bc.Block) {
    // TODO: abort mined block if after 2Delta

    // Deprecated: Genesis block
    if s.db.GetLatest() == nil {
        if block.Index > 0 {
            panic("Local status out of sync")
        }
        // TODO: Signature and broadcast
        s.db.Store(block)
        s.db.UpdateLatest(block)
        s.BroadcastBlock(block, RefererBlock)
    } else {
        // Send miner block into leader gateways
        s.BroadcastBlockToProxies(block)
    }
}
