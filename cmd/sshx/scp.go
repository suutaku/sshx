package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
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
		imp := impl.NewSCP(*srcPath, *destPath, *ident)
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		_, err = sender.Send()
		if err != nil {
			logrus.Error(err)
			return
		}

	}
}
