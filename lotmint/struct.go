package lotmint

/*
This holds the messages used to communicate with the service over the network.
*/

import (
    bc "lotmint/blockchain"

    "go.dedis.ch/onet/v3"
    "go.dedis.ch/onet/v3/network"
)

// Register all messages so the network knows how to handle them.
func init() {
    network.RegisterMessages(
        Count{}, CountReply{},
        Clock{}, ClockReply{},
    )
    network.RegisterMessages(Peer{}, PeerReply{})
    network.RegisterMessages(GenesisBlockRequest{}, BlockByIDRequest{}, BlockByIndexRequest{})
}

const (
    // ErrorParse indicates an error while parsing the protobuf-file.
    ErrorParse = iota + 4000
)

// Clock will run the protocol on the roster and return the time spent doing so.
type Clock struct {
    Roster *onet.Roster
}

// ClockReply returns the time spent for the protocol-run.
type ClockReply struct {
    Time float64
    Children int
}

// Count will return how many times the protocol has been run.
type Count struct {
}

// CountReply returns the number of protocol-runs
type CountReply struct {
    Count int
}

// Peer Operation
type Peer struct {
    Command string
    PeerNodes []*network.ServerIdentity
}

// PeerReply returns the operation status
type PeerReply struct {
    List []*network.ServerIdentity
}

type GenesisBlockRequest struct {
}

type BlockByIDRequest struct {
    Value bc.BlockID
}

type BlockByIndexRequest struct {
    Value int
}
