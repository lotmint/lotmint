package protocol


import (
    "go.dedis.ch/onet/v3"
    "go.dedis.ch/onet/v3/log"
    "go.dedis.ch/onet/v3/network"
)

func init() {
    network.RegisterMessage(Announce{})
    network.RegisterMessage(Reply{})
    _, err := onet.GlobalProtocolRegister(Name, NewProtocol)
    if err != nil {
        panic(err)
    }
}

// TemplateProtocol holds the state of a given protocol.
//
// For this example, it defines a channel that will receive the number
// of children. Only the root-node will write to the channel.
type LotMintProtocol struct {
    *onet.TreeNodeInstance
    announceChan chan announceWrapper
    repliesChan  chan []replyWrapper
    ChildCount   chan int
}

// Check that *LotMintProtocol implements onet.ProtocolInstance
var _ onet.ProtocolInstance = (*LotMintProtocol)(nil)

// NewProtocol initialises the structure for use in one round
func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
    lotMint := &LotMintProtocol{
        TreeNodeInstance: n,
	ChildCount:       make(chan int),
    }
    if err := n.RegisterChannels(&lotMint.announceChan, &lotMint.repliesChan); err != nil {
        return nil, err
    }
    return lotMint, nil
}

// Start sends the Announce-message to all children
func (p *LotMintProtocol) Start() error {
    log.Lvl3(p.ServerIdentity(), "Starting LotMintProtocol")
    return p.SendTo(p.TreeNode(), &Announce{"cothority rulez!"})
}

// Router
// Dispatch implements the main logic of the protocol. The function is only
// called once. The protocol is considered finished when Dispatch returns and
// Done is called.
func (p *LotMintProtocol) Dispatch() error {
    defer p.Done()

    ann := <-p.announceChan
    if p.IsLeaf() {
        return p.SendToParent(&Reply{1})
    }
    p.SendToChildren(&ann.Announce)

    replies := <-p.repliesChan
    n := 1
    for _, c := range replies {
        n += c.ChildrenCount
    }

    if !p.IsRoot() {
        return p.SendToParent(&Reply{n})
    }

    p.ChildCount <- n
    return nil
}
