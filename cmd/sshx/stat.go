package main

import (
	"encoding/gob"

	"github.com/suutaku/sshx/pkg/types"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
)

func cmdStatus(cmd *cli.Cmd) {
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
		conn, err := sender.SendDetach()
		if err != nil {
			logrus.Error(err)
			return
		}
		defer conn.Close()
		logrus.Debug("impl responsed")
		var pld []types.Status
		err = gob.NewDecoder(conn).Decode(&pld)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Debug("show response")
		imp.ShowStatus(pld)
	}
}
