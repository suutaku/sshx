package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/dailer"
	"github.com/suutaku/sshx/internal/node"
	"github.com/suutaku/sshx/internal/proto"
	"github.com/suutaku/sshx/internal/tools"
)

var path = "/etc/sshx"
var port = 22
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
		logrus.Printf("Reading private key file failed %v", err)
		return
	}
	// create signer
	signer, err := tools.SignerFromPem(pemBytes, nil)
	if err != nil {
		logrus.Error(err)
		return
	}
	sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
}

func cmdProxy(cmd *cli.Cmd) {
	cmd.Command("start", "start a proxy", cmdStartProxy)
	cmd.Command("stop", "stop a proxy", cmdStopProxy)
	cmd.Command("list", "list all working proxies", cmdListProxies)
}

func cmdStopProxy(cmd *cli.Cmd) {
	cmd.Spec = "[ -a ] [ ADDR... ]"
	allOption := cmd.BoolOpt("a", false, "destroy all proxy connections")
	addrOption := cmd.StringsArg("ADDR", nil, "destroy proxy connection by IDs or addresses")
	cmd.Action = func() {
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		if allOption != nil && *allOption {
			dal.CloseProxy("")
			return
		}
		if addrOption != nil && len(*addrOption) != 0 {
			for _, v := range *addrOption {
				dal.CloseProxy(v)
			}
		}
	}
}

func cmdListProxies(cmd *cli.Cmd) {
	cmd.Spec = "[ADDR...]"
	addrOption := cmd.StringsArg("ADDR", nil, "destroy proxy connection by IDs or addresses")
	cmd.Action = func() {
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		ret := proto.ListDestroyResponse{}
		if len(*addrOption) == 0 {
			ret = dal.GetProxyList("")
		} else {
			for _, v := range *addrOption {
				ret.List = append(ret.List, dal.GetProxyList(v).List...)
			}
		}
		for _, v := range ret.List {
			fmt.Printf("%v\n", v)
		}

	}
}

func cmdStartProxy(cmd *cli.Cmd) {
	cmd.Spec = "[ -X ] [ -i ] [ -p ] -P [ -d ] ADDR"
	tmp := cmd.BoolOpt("X x11", false, "using X11 opton, default false")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	portTmp := cmd.IntOpt("p", 22, "remote server port")
	proxyPort := cmd.IntOpt("P", 0, "local proxy port")
	detachTmp := cmd.BoolOpt("d", false, "detach proxy connetion, in this case, you need close connetion by call: proxy stop")
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if tmp != nil && *tmp {
			x11 = *tmp
		}
		if ident != nil {
			privateKeyOption(*ident)
		}
		if portTmp != nil {
			port = *portTmp
		}
		if proxyPort == nil || *proxyPort == 0 {
			logrus.Println("please set local proxy port")
			return
		}
		if addr == nil && *addr == "" {
			os.Exit(0)
		}
		userName, address, err := tools.GetParam(*addr)
		if err != nil {
			logrus.Error(err)
			os.Exit(0)
		}
		sshConfig.User = userName
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		req := proto.ConnectRequest{
			Host:      address,
			Port:      int32(port),
			X11:       x11,
			Type:      conf.TYPE_START_PROXY,
			Timestamp: time.Now().Unix(),
			ProxyPort: int32(*proxyPort),
		}
		logrus.Println("proxy set successfully, try ssh at:", req.Host, ":", req.ProxyPort)
		if *detachTmp {
			go func() {

				err = dal.Connect(req, sshConfig)
				if err != nil {
					logrus.Error(err)
					return
				}
				dal.Close(req)
			}()
		} else {
			err = dal.Connect(req, sshConfig)
			if err != nil {
				logrus.Error(err)
			}
			dal.Close(req)
		}
	}
}

func cmdConnect(cmd *cli.Cmd) {

	cmd.Spec = "[ -X ] [ -i ] [ -p ] ADDR"
	tmp := cmd.BoolOpt("X x11", false, "using X11 opton, default false")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	portTmp := cmd.IntOpt("p", 22, "remote host port")
	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if tmp != nil && *tmp {
			x11 = *tmp
		}
		if ident != nil {
			privateKeyOption(*ident)
		}
		if portTmp != nil {
			port = *portTmp
		}
		if addr == nil && *addr == "" {
			os.Exit(0)
		}
		userName, address, err := tools.GetParam(*addr)
		if err != nil {
			logrus.Error(err)
			os.Exit(0)
		}
		sshConfig.User = userName
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		req := proto.ConnectRequest{
			Host:      address,
			Port:      int32(port),
			X11:       x11,
			Type:      conf.TYPE_CONNECTION,
			Timestamp: time.Now().Unix(),
		}
		dal.OpenTerminal(req, sshConfig)
		dal.Close(req)
	}
}

func cmdDaemon(cmd *cli.Cmd) {
	cmd.Action = func() {
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

func cmdCopy(cmd *cli.Cmd) {
	cmd.Spec = "[ -i ] [ -p ] FROM TO"
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	portTmp := cmd.IntOpt("p", 22, "remote host port")
	fromPath := cmd.StringArg("FROM", "", "file path which want to coy")
	toPath := cmd.StringArg("TO", "", "des path ")
	cmd.Action = func() {
		if fromPath == nil || *fromPath == "" {
			os.Exit(1)
		}
		if toPath == nil || *toPath == "" {
			os.Exit(1)
		}

		userName, host, localPath, remotePath, upload, err := tools.ParseCopyParam(*fromPath, *toPath)
		if err != nil {
			logrus.Error(err)
			os.Exit(0)
		}
		sshConfig.User = userName
		if ident != nil {
			privateKeyOption(*ident)
		}
		if portTmp != nil {
			port = *portTmp
		}
		cm := conf.NewConfManager(path)
		dal = dailer.NewDailer(*cm.Conf)
		req := proto.ConnectRequest{
			Host:      host,
			Port:      int32(port),
			Type:      conf.TYPE_CONNECTION,
			X11:       false,
			Timestamp: time.Now().Unix(),
		}
		err = dal.Copy(localPath, remotePath, req, upload, sshConfig)
		if err != nil {
			logrus.Error(err)
		}
		logrus.Println("copy process start to close")
		dal.Close(req)
	}
}

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	home := os.Getenv("SSH_XHOME")
	if home != "" {
		path = home
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// does not exist
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			logrus.Error(err)
			return
		}
	}
	app := cli.App("sshx", "a webrtc based ssh remote tool")
	app.Command("daemon", "launch a sshx daemon", cmdDaemon)
	app.Command("list", "list configure informations", cmdList)
	app.Command("connect", "connect to remote host", cmdConnect)
	app.Command("copy", "cpy files or directories to remote host", cmdCopy)
	app.Command("proxy", "manage proxy", cmdProxy)
	app.Run(os.Args)

}
