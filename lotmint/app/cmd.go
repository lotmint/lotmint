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
	    Description: fmt.Sprint(`
    app peer add HOST:PORT ...
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
}
