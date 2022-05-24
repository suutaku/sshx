package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdStartVNCService(cmd *cli.Cmd) {
	cmd.Spec = "[-P]"
	vncServerPort := cmd.IntOpt("P", 80, "local proxy port")
	cmd.Action = func() {
		imp := impl.NewVNCService(int32(*vncServerPort))
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		if sender == nil {
			logrus.Error("can not create impl")
			return
		}
		_, err = sender.SendDetach()
		if err != nil {
			logrus.Error(err)
			return
		}
	}
}

func cmdStopVNCService(cmd *cli.Cmd) {
	cmd.Spec = "PID"
	pidOpt := cmd.StringArg("PID", "", "vnc server pair Id")
	cmd.Action = func() {
		if pidOpt == nil || *pidOpt == "" {
			return
		}
		sender := impl.NewSender(&impl.VNCService{}, types.OPTION_TYPE_DOWN)
		sender.PairId = []byte(*pidOpt)
		sender.SendDetach()
	}
}

func cmdVNCService(cmd *cli.Cmd) {
	cmd.Command("start", "start vnc service", cmdStartVNCService)
	cmd.Command("stop", "stop vnc service", cmdStopVNCService)
}
