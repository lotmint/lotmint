/* Websocket */
package service

import (
	"time"

	"errors"
	// "fmt"
	"sync"

	"lotmint"
	"lotmint/protocol"
	"go.dedis.ch/onet/v3"
	"go.dedis.ch/onet/v3/log"
	"go.dedis.ch/onet/v3/network"
        "golang.org/x/xerrors"
)

var serviceID onet.ServiceID
var peers []string


func init() {
    var err error
    serviceID, err = onet.RegisterNewService(lotmint.ServiceName, newService)
    log.ErrFatal(err)
    network.RegisterMessage(&storage{})
}

type Service struct {
    // We need to embed the ServiceProcessor, so that incoming messages
    // are correctly handled.
    *onet.ServiceProcessor

    storage *storage

    peerStorage *peerStorage
}

var storageID = []byte("main")

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
    addPeerChan chan []string
    delPeerChan chan []string
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
    var peers []string
    var resp = &lotmint.PeerReply{}
    switch req.Command {
    case "add":
	for _, peerNode := range req.PeerNodes{
	    if _, ok := s.peerStorage.peerNodeMap[peerNode]; !ok {
                peers = append(peers, peerNode)
	    }
	}
	if len(peers) > 0 {
            s.peerStorage.addPeerChan <- peers
        }
    case "del":
	for _, peerNode := range req.PeerNodes{
	    if _, ok := s.peerStorage.peerNodeMap[peerNode]; ok {
                peers = append(peers, peerNode)
	    }
	}
	if len(peers) > 0 {
            s.peerStorage.delPeerChan <- peers
        }
    case "show":
	if len(req.PeerNodes) > 0 {
	    peers = req.PeerNodes
	} else {
	    for key, _ := range s.peerStorage.peerNodeMap {
                peers = append(peers, key)
            }
	}
    default:
        return nil, xerrors.New("Command not supported")
    }
    resp.List =  peers
    return resp, nil
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
    return nil
}

func (s *Service) AddPeerServerIdentity(peers []string) {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()
}

func (s *Service) RemovePeerServerIdentity(peers []string) {
    s.peerStorage.Lock()
    defer s.peerStorage.Unlock()

}

func (s *Service) loop() {
    for {
        select {
	case peers := <-s.peerStorage.addPeerChan:
		go s.AddPeerServerIdentity(peers)
        case peers := <-s.peerStorage.delPeerChan:
		go s.RemovePeerServerIdentity(peers)
	}
    }
}

// newService receives the context that holds information about the node it's
// running on. Saving and loading can be done using the context. The data will
// be stored in memory for tests and simulations, and on disk for real deployments.
func newService(c *onet.Context) (onet.Service, error) {
    s := &Service{
        ServiceProcessor: onet.NewServiceProcessor(c),
	peerStorage: &peerStorage{
	    peerNodeMap: make(map[string]*network.ServerIdentity),
	    addPeerChan: make(chan []string),
	    delPeerChan: make(chan []string),
	},
    }
    if err := s.RegisterHandlers(s.Clock, s.Count); err != nil {
        return nil, errors.New("Couldn't register handlers")
    }
    if err := s.RegisterHandlers(s.Peer); err != nil {
        return nil, errors.New("Couldn't register handlers")
    }
    if err := s.tryLoad(); err != nil {
        log.Error(err)
	return nil, err
    }
    go s.loop()
    return s, nil
}
