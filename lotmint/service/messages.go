package service

func init() {
    network.RegisterMessages(GenesisBlockRequest{}, GenesisBlockReply{})
}
