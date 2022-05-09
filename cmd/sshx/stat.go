package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/impl"
)

func cmdStatus(cmd *cli.Cmd) {
	// cmd.Spec = "[-t]"
	// typeFilter := cmd.StringOpt("t type","","application type filter")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewStatImpl()
		defer dialer.Close()
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dialer.Init(param)
		err := dialer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
	}
}
