package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
	"github.com/suutaku/sshx/pkg/conf"
)

func cmdCopy(cmd *cli.Cmd) {
	cmd.Spec = "[ -i ] SRC DEST"
	srcPath := cmd.StringArg("SRC", "", "[username]@[host]:/path")
	destPath := cmd.StringArg("DEST", "", "[username]@[host]:/path")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	cmd.Action = func() {
		if srcPath == nil || *destPath == "" {
			return
		}
		if destPath == nil || *destPath == "" {
			return
		}
		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewScpImpl()

		// init dialer
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dialer.Init(param)

		err := dialer.ParsePaths(*srcPath, *destPath)
		if err != nil {
			logrus.Error(err)
			return
		}
		// parse client option
		dialer.PrivateKeyOption(*ident)
		logrus.Debug("cmd connect")
		err = dialer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
		dialer.Close()
	}
}
