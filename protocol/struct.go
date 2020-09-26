package protocol

import "go.dedis.ch/onet/v3"

// Name can be used from other packages to refer to this protocol.
const Name = "LotMintProtocol"

// Announce is used to pass a message to all children.
type Announce struct {
    Message string
}

// announceWrapper just contains Announce and the data necessary to identify
// and process the message in onet.
type announceWrapper struct {
    *onet.TreeNode
    Announce
}

// Reply returns the count of all children.
type Reply struct {
    ChildrenCount int
}

// replyWrapper just contains Reply and the data necessary to identify and
// process the message in onet.
type replyWrapper struct {
    *onet.TreeNode
    Reply
}
