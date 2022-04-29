package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
	"github.com/suutaku/sshx/pkg/conf"
)

func cmdConnect(cmd *cli.Cmd) {
	cmd.Spec = "[ -X ] [ -i ] ADDR"
	tmp := cmd.BoolOpt("X x11", false, "using X11 opton, default false")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")

	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if addr == nil || *addr == "" {
			return
		}
		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewSshImpl()

		// init dialer
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dialer.Init(param)

		// parse user name
		err := dialer.DecodeAddress(*addr)
		if err != nil {
			logrus.Error(err)
			return
		}
		// parse client option
		dialer.PrivateKeyOption(*ident)
		// parse x11 option
		dialer.X11Option(*tmp)
		logrus.Debug("cmd connect")
		err = dialer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
		dialer.Close()
	}
}
