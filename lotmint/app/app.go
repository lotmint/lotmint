package main

import (
    "errors"
    "net/url"
    "os"
    "path"

    lotmint "lotmint"

    "go.dedis.ch/onet/v3/app"
    "go.dedis.ch/onet/v3/cfgpath"
    "go.dedis.ch/onet/v3/log"
    "go.dedis.ch/onet/v3/network"
    "go.dedis.ch/kyber/v3/suites"
    "go.dedis.ch/kyber/v3/util/encoding"
    "golang.org/x/xerrors"
    "gopkg.in/urfave/cli.v1"
)

const (
	// DefaultName is the name of the binary we produce and is used to create a directory
	// folder with this name
	DefaultName = "lotmint"
)

func main() {
    cliApp := cli.NewApp()
    cliApp.Name = "app"
    cliApp.Usage = "Used for building other apps."
    cliApp.Version = "0.1"
    groupsDef := "the group-definition-file"
    cliApp.Commands = []cli.Command{
        {
            Name:	"time",
	    Usage:	"measure the time to contact all nodes.",
	    Aliases:	[]string{"t"},
	    ArgsUsage:	groupsDef,
	    Action:	cmdTime,
	},
        {
            Name:	"counter",
	    Usage:	"return the counter",
	    Aliases:	[]string{"c"},
	    ArgsUsage:	groupsDef,
	    Action:	cmdCounter,
	},
    }
    cliApp.Commands = append(cliApp.Commands, cmds...)
    cliApp.Flags = []cli.Flag{
        cli.IntFlag{
            Name: "debug, d",
	    Value: 0,
	    Usage: "debug-level: 1 for terse, 5 for maximal,",
	},
	cli.StringFlag{
            Name: "config, c",
            Value: path.Join(cfgpath.GetConfigPath(DefaultName), app.DefaultGroupFile),
            Usage: "Configuration file of the server",
	},
    }
    cliApp.Before = func(c *cli.Context) error {
        log.SetDebugVisible(c.Int("debug"))
	return nil
    }
    log.ErrFatal(cliApp.Run(os.Args))
}

// Use readGroup instead later.
func parseConfig(c *cli.Context) *app.Group {
    config := c.GlobalString("config")
    if _, err := os.Stat(config); os.IsNotExist(err) {
        log.Fatalf("[-] Configuration file does not exist. %s", config)
    }
    f, err := os.Open(config)
    log.ErrFatal(err, "Couldn't open group definition file")
    group, err := app.ReadGroupDescToml(f)
    log.ErrFatal(err, "Error while reading group definition file", err)
    if len(group.Roster.List) == 0 {
        log.ErrFatalf(err, "Empty entity or invalid group definition in: %s", config)
    }
    return group
}

// Convert url string info peer struct
func convertPeerURL(peerURL string) (*network.ServerIdentity, error) {
    parse, err := url.Parse(peerURL)
    if err != nil {
        return nil, xerrors.Errorf("url parse error: %v", err)
    }
    suite, err := suites.Find("Ed25519")
    if err != nil {
        return nil, xerrors.Errorf("kyber suite: %v", err)
    }
    point, err := encoding.StringHexToPoint(suite, parse.User.Username())
    if err != nil {
	return nil, xerrors.Errorf("parsing public key: %v", err)
    }
    var connType network.ConnType
    switch parse.Scheme {
    case "tcp":
	connType = network.PlainTCP
    default:
        connType = network.TLS
    }
    si := network.NewServerIdentity(point, network.NewAddress(connType, parse.Host))
    return si, nil
}

// Add new peer node to peer storage
func addPeer(c *cli.Context) error {
    if c.NArg() < 1 {
	    return xerrors.New("please give the following arguments: " +
	        "host[:port] [host[:port]]...")
    }
    var peers []*network.ServerIdentity
    for i := 0; i < c.NArg(); i++ {
	peerURL := c.Args().Get(i)
	si, err := convertPeerURL(peerURL)
	if err == nil {
            peers = append(peers, si)
        }
    }
    log.Info("Add peers:", peers)
    group := parseConfig(c)
    client := lotmint.NewClient()
    resp, err := client.Peer(group.Roster, &lotmint.Peer{
            Command: "add",
	    PeerNodes: peers,
    })
    if err != nil {
	    return errors.New("Error: " + err.Error())
    }
    log.Info("Finished to add peers", resp.List)
    return nil
}

// Delete new peer node from peer storage
func delPeer(c *cli.Context) error {
    if c.NArg() < 1 {
	    return xerrors.New("please give the following arguments: " +
	        "host[:port] [host[:port]]...")
    }
    var peers []*network.ServerIdentity
    for i := 0; i < c.NArg(); i++ {
	peerURL := c.Args().Get(i)
	si, err := convertPeerURL(peerURL)
	if err == nil {
            peers = append(peers, si)
        }
    }
    log.Info("Remove peers: ", peers)
    group := parseConfig(c)
    client := lotmint.NewClient()
    resp, err := client.Peer(group.Roster, &lotmint.Peer{
            Command: "del",
	    PeerNodes: peers,
    })
    if err != nil {
	    return errors.New("Error: " + err.Error())
    }
    log.Info("Finished to remove peers", resp.List)
    return nil
}

// Show peer status
func showPeer(c *cli.Context) error {
    var peers []*network.ServerIdentity
    for i := 0; i < c.NArg(); i++ {
	peerURL := c.Args().Get(i)
	si, err := convertPeerURL(peerURL)
	if err == nil {
            peers = append(peers, si)
        }
    }
    if len(peers) > 0 {
        log.Info("Show peers: ", peers)
    } else {
        log.Info("Show all peers")
    }
    group := parseConfig(c)
    client := lotmint.NewClient()
    resp, err := client.Peer(group.Roster, &lotmint.Peer{
            Command: "show",
	    PeerNodes: peers,
    })
    if err != nil {
	    return errors.New("Error: " + err.Error())
    }
    log.Info("Peers Status\n", resp.List)
    return nil
}

// Returns the time needed to contact all nodes.
func cmdTime(c *cli.Context) error {
    log.Info("Time command")
    group := readGroup(c)
    client := lotmint.NewClient()
    resp, err := client.Clock(group.Roster)
    if err != nil {
	    return errors.New("When asking the time: " + err.Error())
    }
    log.Infof("Children: %d - Time spent: %f", resp.Children, resp.Time)
    return nil
}

// Returns the number of calls
func cmdCounter(c *cli.Context) error {
    log.Info("Counter command")
    group := readGroup(c)
    client := lotmint.NewClient()
    counter, err := client.Count(group.Roster.RandomServerIdentity())
    if err != nil {
        return errors.New("When asking for counter: " + err.Error())
    }
    log.Info("Number of requests:", counter)
    return nil
}

func readGroup(c *cli.Context) *app.Group {
    if c.NArg() != 1 {
        log.Fatal("Please give the group-file as argument")
    }
    name := c.Args().First()
    f, err := os.Open(name)
    log.ErrFatal(err, "Couldn't open group definition file")
    group, err := app.ReadGroupDescToml(f)
    log.ErrFatal(err, "Error while reading group definition file", err)
    if len(group.Roster.List) == 0 {
        log.ErrFatalf(err, "Empty entity or invalid group definition in: %s", name)
    }
    return group
}
