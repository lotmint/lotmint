package main

import (
    "github.com/urfave/cli"
)

const (
    DefaultName = "test"
)

var cliApp = cli.NewApp()

// getDataPath is a function pointer so that tests can hook and modify this.
var getDataPath = cfgpath.GetDataPath

var gitTag = "dev"

// ConfigPath points to where the files will be stored by default.
var ConfigPath = "."

func init() {
    cliApp.Name = DefaultName
    cliApp.Usage = "Test"
    cliApp.Version = gitTag
    cliApp.Action = run
    cliApp.Commands = []
    cliApp.Flags = []cli.Flag{
        cli.IntFlags{
            Name: "debug, d",
	    Value: 0,
	    Usage: "debug-level: 1 for terse, 5 for maximal",
	},
	cli.StringFlag{
            Name: "config, c",
	    EnvVar: "BC_CONFIG",
	    Value: getDataPath("test"),
	    Usage: "path to configuration-directory",
	},
	cli.BoolFlag{
            Name: "wait, w",
	    EnvVar: "BC_WAIT",
	    Usage: "wait for transaction available in all nodes",
	}
    }
    cliApp.Before = func(c *cli.Context) error {
        log.SetDebugVisible(c.Int("debug"))
	ConfigPath = c.String("config")
    }
}

func run(c *cli.Context) error {
    fn := c.Args().Get(2)
    if fn == "" {
        err = xerrors.New("no TOML file provided")
	return
    }
    f, err := os.Open(fn)
    if err != nil {
        return
    }
    defer f.Close()
    group, err := app.ReadGroupDescToml(f)
    if err != nil {
        err = xerrors.Errorf("clouldn't open %v: %v", fn, err)
	return
    }
    group.Roster
    // first check the options
    config := ctx.GlobalString("config")
    log.Info("Creating Self-Defined Transaction")
    client := byzcoin.NewClient(id, roster)
    ctx, err := client.CreateTransaction(byzcoin.Instruction {
        InstanceID: byzcoin.NewInstanceID(),
        Spawn: &byzcoin.Spawn {
            ContractID: contracts.ContractCoinID,
	    Args: byzcoin.Arguments {{
                Name: "test",
	        Value: "test"
            }},
        },
    })
    if err != nil {
        return err
    }
    _, err = client.AddTransactionAndWait(ctx, 10)
    if err != nil {
        return err
    }

    log.Infof("Account %x created", account[:])

    return lib.WaitPropagation(c, client)
}


func main() {
    err := cliApp.Run(os.Args)
    log.Fatalf(err)
}
