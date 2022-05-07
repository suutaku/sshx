package main

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
	"github.com/suutaku/sshx/pkg/conf"
)

func cmdStopProxy(cmd *cli.Cmd) {
	cmd.Spec = "PID"
	pairId := cmd.StringArg("PID", "", "Connection pair id which can found by using status command")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewProxyImpl()
		param := impl.ImplParam{
			Config: *cm.Conf,
			PairId: *pairId,
		}
		dialer.Init(param)
		dialer.Close()
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
		fmt.Println("Press Ctrl+C to close proxy")

		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewProxyImpl()

		// init dialer
		param := impl.ImplParam{
			Config: *cm.Conf,
			HostId: *addr,
		}
		dialer.Init(param)
		dialer.SetProxyPort(int32(*proxyPort))
		if err := dialer.Dial(); err != nil {
			logrus.Debug(err)
		}

	}
}

func cmdProxy(cmd *cli.Cmd) {
	cmd.Command("start", "start proxy service", cmdStartProxy)
	cmd.Command("stop", "stop proxy service", cmdStopProxy)
}
