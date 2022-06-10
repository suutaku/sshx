package main

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdStartMessage(cmd *cli.Cmd) {
	// cmd.Spec = "-P [-d] ADDR"
	cmd.Spec = "ADDR"
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if addr == nil || *addr == "" {
			fmt.Println("please set a remote device")
		}

		msgr := impl.NewMessager(*addr)
		msgr.Preper()

		sender := impl.NewSender(msgr, types.OPTION_TYPE_UP)
		conn, err := sender.Send()
		if err != nil {
			logrus.Error("cannot create messager with ", *addr, ":", err)
			return
		}
		msgr.OpenChatConsole(conn)
		msgr.Close()
	}
}
