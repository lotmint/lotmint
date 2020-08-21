package main

import (
    "fmt"

    "gopkg.in/urfave/cli.v1"
)

var cmds = cli.Commands{
    {
            Name:	"peer",
	    Usage:	"Provides cli interface for peers.",
	    Aliases:	[]string{"p"},
	    ArgsUsage: "<transaction name> [<arg>...]",
	    Description: fmt.Sprint(`
    app peer add HOST:PORT ...
    app peer del HOST:PORT ...
    app peer show
	    `),
	    Subcommands: cli.Commands{
                {
                    Name: "add",
	            Usage:  "Add new peers",
	            ArgsUsage: "Peer Add",
	            Action: addPeer,
		},
                {
                    Name: "del",
	            Usage:  "Remove new peers",
	            ArgsUsage: "Peer Del",
	            Action: delPeer,
		},
                {
                    Name: "show",
	            Usage:  "Show all peers status",
	            ArgsUsage: "Peer Status",
	            Action: showPeer,
		},
	    },
    },
    {
            Name:	"genesisblock",
	    Usage:	"Initialize new blockchain.",
	    Aliases:	[]string{"g"},
	    ArgsUsage: "<transaction name> [<arg>...]",
	    Action: createGenesisBlock,
    },
    {
            Name:	"block",
	    Usage:	"Read a block given by an index or hash id",
	    Aliases:	[]string{"b"},
	    Action: showBlock,
	    Flags: []cli.Flag{
                cli.IntFlag{
                    Name: "index",
		    Value: -1,
		    Usage: "give this block index",
		},
                cli.StringFlag{
                    Name: "hash",
		    Usage: "give block hash id to show",
		},
	    },
    },
}
