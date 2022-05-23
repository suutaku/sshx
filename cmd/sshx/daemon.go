package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/suutaku/sshx/internal/node"
)

func cmdDaemon(cmd *cli.Cmd) {
	cmd.Action = func() {
		n := node.NewNode(getRootPath())
		defer n.Stop()
		n.Start()
	}
}
