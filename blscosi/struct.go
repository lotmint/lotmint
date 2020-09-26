package blscosi

import (
    "time"

    "go.dedis.ch/onet/v3"
)

const DefaultProtocolName = "blsCoSiProtocol"

// Announcement is the blscosi annoucement message.
type Announcement struct {
    Msg       []byte // statement to be signed
    Timeout   time.Duration
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the onet framework.
type StructAnnouncement struct {
    *onet.TreeNode
    Announcement
}

// Response is the blscosi response message.
type Response struct {
    //Signature BlsSignature
    //Mask      []byte
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the onet framework.
type StructResponse struct {
    *onet.TreeNode
    Response
}

type ChannelResponse struct {
    Responses []*Response
}
