package blscosi

import (
    "time"

    "go.dedis.ch/kyber/v3/pairing"
    "go.dedis.ch/onet/v3"
    "go.dedis.ch/onet/v3/log"
    "golang.org/x/xerrors"
)

type BlsCosi struct {
    *onet.TreeNodeInstance
    suite               *pairing.SuiteBn256
    ChannelAnnouncement chan StructAnnouncement
    ChannelResponse     chan StructResponse
    ResponseChan        chan ChannelResponse
}

func init() {
    onet.GlobalProtocolRegister(DefaultProtocolName, NewProtocol)
}

func NewProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
    return NewBlsCosi(n, pairing.NewSuiteBn256())
}
// Dispatch runs the protocol for each node in the protocol acting according
// to its type
func (p *BlsCosi) Dispatch() error {
    defer p.Done()

    ann := <-p.ChannelAnnouncement
    //p.ServerIdentity().Public.String()
    if !p.IsRoot() {
        r, err := p.makeResponse()
	if err != nil {
	    return err
	}
        return p.SendToParent(r)
    }
    errs := p.SendToChildrenInParallel(ann.Announcement)
    if len(errs) > 0 {
	log.Error(errs)
    }

    var responses []*Response
    timeout := time.After(5)
    done := len(errs)
    for done < len(p.Children()) {
        select {
        case reply := <-p.ChannelResponse:
	    responses = append(responses, &reply.Response)
            done++
	case <-timeout:
            log.Lvlf3("Subleader reached timeout waiting for children responses: %v", p.ServerIdentity())
	    // Use whatever we received until then to try to finish
	    // the protocol
	    done = len(p.Children())
        }
    }

    p.ResponseChan <- ChannelResponse{responses}

    return nil
}

// Start sends the Announce-message to all children
func (p *BlsCosi) Start() error {
    log.Lvl3(p.ServerIdentity(), "Starting LotMintProtocol")
    return p.SendTo(p.TreeNode(), &Announcement{[]byte("cothority rulez!"), 10})
}

func NewBlsCosi(n *onet.TreeNodeInstance, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
    c := &BlsCosi {
        TreeNodeInstance:  n,
	suite:             suite,
    }
    err := c.RegisterChannels(&c.ChannelAnnouncement, &c.ChannelResponse)
    if err != nil {
	return nil, xerrors.New("couldn't register channels: " + err.Error())
    }
    return c, nil
}

// Sign the message
func (p *BlsCosi) makeResponse() (*Response, error) {
    return &Response{
        //Mask:      mask.Mask(),
	//Signature: sig,
    }, nil
}
