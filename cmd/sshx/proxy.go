package main

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdStopProxy(cmd *cli.Cmd) {
	cmd.Spec = "PID"
	pairId := cmd.StringArg("PID", "", "Connection pair id which can found by using status command")
	cmd.Action = func() {
		sender := impl.NewSender(&impl.Proxy{}, types.OPTION_TYPE_DOWN)
		sender.PairId = []byte(*pairId)
		sender.SendDetach()
	}
}

func cmdStartProxy(cmd *cli.Cmd) {
	// cmd.Spec = "-P [-d] ADDR"
	cmd.Spec = "-P ADDR"
	proxyPort := cmd.IntOpt("P", 0, "local proxy port")
	// detach := cmd.BoolOpt("d", false, "detach process")
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if proxyPort == nil || *proxyPort == 0 {
			fmt.Println("please set a proxy port")
		}
		if addr == nil || *addr == "" {
			fmt.Println("please set a remote device")
		}

		proxy := impl.NewProxy(int32(*proxyPort), *addr)
		proxy.Preper()

		sender := impl.NewSender(proxy, types.OPTION_TYPE_UP)
		_, err := sender.SendDetach()
		if err != nil {
			logrus.Error(err)
		}
	}
}

func cmdProxy(cmd *cli.Cmd) {
	cmd.Command("start", "start proxy service", cmdStartProxy)
	cmd.Command("stop", "stop proxy service", cmdStopProxy)
}
