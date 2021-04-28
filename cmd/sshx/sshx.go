package main

import (
	"context"
	"flag"
	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/node"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	var target string
	var help bool
	var deamon bool
	var list bool
	var create bool
	log.SetFlags(log.Lshortfile)
	flag.StringVar(&target, "t", "", "set/reset target id")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&deamon, "d", false, "run sshx as a deamon")
	flag.BoolVar(&list, "l", false, "show configure info")
	flag.BoolVar(&create, "c", false, "create a default configure file")
	flag.Usage = func() {
		flag.Args()
	}
	flag.Parse()

	path := "."
	home := os.Getenv("HOME")
	if home != "" {
		path = home
	}

	switch {
	case help:
		flag.PrintDefaults()
		return
	case deamon:
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT)
		ctx, cancel := context.WithCancel(context.Background())
		cm := conf.NewConfManager(path)
		node := node.NewNode(cm.Conf)
		node.Start(ctx)
		<-sig
		cancel()
	case list:
		cm := conf.NewConfManager(path)
		cm.Show()
		return
	case create:
		cm := conf.NewConfManager(path)
		cm.Show()
		return
	case target != "":
		cm := conf.NewConfManager(path)
		cm.Set("key", target)
		return
	default:
		flag.PrintDefaults()
		return
	}
}
