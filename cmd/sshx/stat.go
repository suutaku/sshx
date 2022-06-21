package main

import (
	"github.com/suutaku/sshx/pkg/types"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
)

func cmdStatus(cmd *cli.Cmd) {
	cmd.Spec = "[ -t ]"
	treeOpt := cmd.BoolOpt("t", false, "display in tree view")
	cmd.Action = func() {
		imp := impl.NewSTAT()
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}

		sender := impl.NewSender(imp, types.OPTION_TYPE_STAT)
		if sender == nil {
			logrus.Error("cannot create sender")
			return
		}
		conn, err := sender.Send()
		if err != nil {
			logrus.Error(err)
			return
		}
		imp.SetConn(conn)
		logrus.Debug("impl responsed")
		displayStyle := impl.DISPLAY_TABLE
		if *treeOpt {
			displayStyle = impl.DISPLAY_TREE
		}
		imp.ShowStatus(displayStyle)
		imp.Close()
	}
}
