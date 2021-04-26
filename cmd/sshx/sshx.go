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
	log.SetFlags(log.Lshortfile)
	flag.StringVar(&target, "t", "-", "set/reset target id")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&deamon, "d", false, "run sshx as a deamon")
	flag.Usage = func() {
		flag.Args()
	}
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	path := "."
	home := os.Getenv("HOME")
	if home != "" {
		path = home
	}
	log.Println("target")
	log.Println(target)
	log.Println(path)
	if target != "-" {
		cm := conf.NewConfManager(path)
		cm.Set("key", target)
		return
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	cm := conf.NewConfManager(path)
	node := node.NewNode(cm.Conf)
	node.Start(ctx)
	<-sig
	cancel()
}
