package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	cli "github.com/jawher/mow.cli"
	"golang.org/x/crypto/ssh"

	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/dailer"
	"github.com/suutaku/sshx/internal/node"
	"github.com/suutaku/sshx/internal/tools"
)

var path = "/etc/sshx"
var dal *dailer.Dailer
var sshConfig = ssh.ClientConfig{
	// User:            user,
	// Auth:            []ssh.AuthMethod{ssh.Password(pass)},
	Auth:            []ssh.AuthMethod{},
	HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	Timeout:         10 * time.Second,
}
var x11 = false

func cmdList(cmd *cli.Cmd) {
	cmd.Action = func() {
		cm := conf.NewConfManager(path)
		cm.Show()
	}
}
func privateKeyOption(keyPath string) {
	if keyPath == "" {
		home := os.Getenv("HOME")
		keyPath = home + "/.ssh/id_rsa"
	}
	pemBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Printf("Reading private key file failed %v", err)
		os.Exit(0)
	}
	// create signer
	signer, err := tools.SignerFromPem(pemBytes, nil)
	if err != nil {
		log.Println(err)
		os.Exit(0)
	}
	sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
}

func cmdConnect(cmd *cli.Cmd) {

	cmd.Spec = "[ -X ] [ -i ] ADDR"
	tmp := cmd.BoolOpt("X x11", false, "using X11 opton, default false")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if tmp != nil && *tmp {
			x11 = *tmp
		}
		if ident != nil {
			privateKeyOption(*ident)
		}
		if addr == nil && *addr == "" {
			os.Exit(0)
		}
		userName, address, port, err := tools.GetParam(*addr)
		if err != nil {
			log.Println(err)
			os.Exit(0)
		}
		sshConfig.User = userName
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		err = dal.Connect(address, port, x11, sshConfig)
		if err != nil {
			log.Println(err)
			dal.Close()
			os.Exit(0)
		}
		dal.Close()
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
	home := os.Getenv("SSH_XHOME")
	if home != "" {
		path = home
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// does not exist
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			log.Println(err)
			return
		}
	}
	app := cli.App("sshx", "a webrtc based ssh remote tool")
	app.Command("daemon", "launch a sshx daemon", cmdDaemon)
	app.Command("list", "list configure informations", cmdList)
	app.Command("connect", "connect to remove device", cmdConnect)
	app.Run(os.Args)

}
