package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	cli "github.com/jawher/mow.cli"

	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/dailer"
	"github.com/suutaku/sshx/internal/node"
	"github.com/suutaku/sshx/internal/tools"
)

var path = "."
var dal *dailer.Dailer

func cmdList(cmd *cli.Cmd) {
	cmd.Action = func() {
		cm := conf.NewConfManager(path)
		cm.Show()
	}
}

func cmdConnect(cmd *cli.Cmd) {
	cmd.Spec = "ADDR"
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if addr == nil && *addr == "" {
			return
		}
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		defer dal.Close()
		userName, address, port, err := tools.GetParam(*addr)
		if err != nil {
			log.Println(err)
			return
		}
		err = dal.Connect(userName, address, port)
		if err != nil {
			panic(err)
		}
	}
}

func cmdDaemon(cmd *cli.Cmd) {
	cmd.Action = func() {
		log.Println("run as a daemon")
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT)
		ctx, cancel := context.WithCancel(context.Background())
		cm := conf.NewConfManager(path)
		node := node.NewNode(cm.Conf)
		node.Start(ctx)
		<-sig
		cancel()
	}
}

func main() {
	log.SetFlags(log.Lshortfile)
	home := os.Getenv("HOME")
	if home != "" {
		path = home
	}
	app := cli.App("sshx", "a webrtc based ssh remote tool")
	app.Command("daemon", "launch a sshx daemon", cmdDaemon)
	app.Command("list", "list configure informations", cmdList)
	app.Command("connect", "connect to remove device", cmdConnect)
	app.Run(os.Args)

}
