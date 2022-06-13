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

func cmdAttachMessage(cmd *cli.Cmd) {
	// cmd.Spec = "-P [-d] ADDR"
	cmd.Spec = "PID"
	pairId := cmd.StringArg("PID", "", "Connection pair id which can found by using status command")
	cmd.Action = func() {
		if pairId == nil || *pairId == "" {
			fmt.Println("please set a pair id")
		}

		msgr := impl.NewMessager("")
		msgr.Preper()

		sender := impl.NewSender(msgr, types.OPTION_TYPE_ATTACH)
		sender.PairId = []byte(*pairId)
		conn, err := sender.Send()
		if err != nil {
			logrus.Error("cannot attach messager with ", *pairId, ":", err)
			return
		}
		updateMsgr := sender.GetImpl().(*impl.Messager)
		updateMsgr.OpenChatConsole(conn)
		updateMsgr.Close()
	}
}

func cmdMessage(cmd *cli.Cmd) {
	cmd.Command("start", "start message service", cmdStartMessage)
	cmd.Command("attach", "attach a message service service", cmdAttachMessage)
	// cmd.Command("stop", "stop proxy service", cmdStopMessage)
}
