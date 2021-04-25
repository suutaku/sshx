package main

import (
	"context"
	"github.com/suutaku/sshx/internal/node"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	path := "."
	home := os.Getenv("HOME")
	if home != "" {
		path = home
	}
	node := node.NewNode(path)
	node.Start(ctx)
	<-sig
	cancel()
}
