package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
	"github.com/suutaku/sshx/internal/node"
	"github.com/suutaku/sshx/pkg/conf"
)

var defaultHomePath = "/etc/sshx"

func getRootPath() string {
	rootStr := os.Getenv("SSHX_HOME")
	if rootStr == "" {
		rootStr = defaultHomePath
	}
	if _, err := os.Stat(rootStr); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(rootStr, 0766)
		if err != nil {
			logrus.Error(err)
		}
	}
	return rootStr
}

func cmdGetConfig(cmd *cli.Cmd) {
	cmd.Spec = "[KEYS...]"
	keys := cmd.StringsArg("KEYS", nil, "get cofigure by key [[key1] [key2]],[key1.key2]. if key is empty, list all configure info")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		if keys == nil || len(*keys) == 0 {
			cm.Show()
			return
		}
		for _, v := range *keys {
			res := cm.Viper.Get(v)
			fmt.Printf("%s:\t%#v\n", v, res)
		}
	}
}

func cmdSetConfig(cmd *cli.Cmd) {
	cmd.Spec = "KEY VALUE"
	key := cmd.StringArg("KEY", "", "configure key, [key] ]value], [key1.key2] [value]")
	value := cmd.StringArg("VALUE", "", "configure value")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		if key == nil || *key == "" {
			return
		}
		if value == nil || *value == "" {
			return
		}
		cm.Set(*key, *value)
	}
}

func cmdConfig(cmd *cli.Cmd) {
	cmd.Command("set", "set configure with key value", cmdSetConfig)
	cmd.Command("get", "get configure value with key", cmdGetConfig)
}
func cmdDaemon(cmd *cli.Cmd) {
	cmd.Action = func() {
		n := node.NewNode(getRootPath())
		n.Start()
	}
}

func cmdConnect(cmd *cli.Cmd) {
	cmd.Spec = "[ -X ] [ -i ] ADDR"
	tmp := cmd.BoolOpt("X x11", false, "using X11 opton, default false")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")

	addr := cmd.StringArg("ADDR", "", "remote target address [username]@[host]:[port]")
	cmd.Action = func() {
		if addr == nil || *addr == "" {
			return
		}
		cm := conf.NewConfManager(getRootPath())
		dailer := impl.NewSshImpl()

		// init dailer
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dailer.Init(param)

		// parse user name
		err := dailer.DecodeAddress(*addr)
		if err != nil {
			logrus.Error(err)
			return
		}
		// parse client option
		dailer.PrivateKeyOption(*ident)
		// parse x11 option
		dailer.X11Option(*tmp)
		logrus.Debug("cmd connect")
		err = dailer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
		dailer.Close()
	}
}

func cmdMount(cmd *cli.Cmd) {}

func cmdStatus(cmd *cli.Cmd) {
	// cmd.Spec = "[-t]"
	// typeFilter := cmd.StringOpt("t type","","application type filter")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		dailer := impl.NewStatImpl()
		defer dailer.Close()
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dailer.Init(param)
		err := dailer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
	}
}

func cmdCopy(cmd *cli.Cmd) {
	cmd.Spec = "[ -i ] SRC DEST"
	srcPath := cmd.StringArg("SRC", "", "[username]@[host]:/path")
	destPath := cmd.StringArg("DEST", "", "[username]@[host]:/path")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	cmd.Action = func() {
		if srcPath == nil || *destPath == "" {
			return
		}
		if destPath == nil || *destPath == "" {
			return
		}
		cm := conf.NewConfManager(getRootPath())
		dailer := impl.NewScpImpl()

		// init dailer
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dailer.Init(param)

		err := dailer.ParsePaths(*srcPath, *destPath)
		if err != nil {
			logrus.Error(err)
			return
		}
		// parse client option
		dailer.PrivateKeyOption(*ident)
		logrus.Debug("cmd connect")
		err = dailer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}
		dailer.Close()
	}
}

func cmdStartProxy(cmd *cli.Cmd) {
	cmd.Spec = "-P [-d] ADDR"
	proxyPort := cmd.IntOpt("P", 0, "local proxy port")
	detach := cmd.BoolOpt("d", false, "detach process")
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
		dailer := impl.NewProxyImpl()

		// init dailer
		param := impl.ImplParam{
			Config: *cm.Conf,
			HostId: *addr,
		}
		dailer.Init(param)
		dailer.SetProxyPort(int32(*proxyPort))
		if *detach {
			go func() {
				if err := dailer.Dial(); err != nil {
					logrus.Debug(err)
				}
			}()
			return
		}
		if err := dailer.Dial(); err != nil {
			logrus.Debug(err)
		}

	}
}

func cmdProxy(cmd *cli.Cmd) {
	cmd.Command("start", "start a proxy", cmdStartProxy)
}

func debugOn() bool {
	str := os.Getenv("SSHX_DEBUG")
	if str == "" {
		return false
	}
	lowStr := strings.ToLower(str)
	if lowStr == "1" || lowStr == "true" || lowStr == "yes" {
		return true
	}
	return false
}

func main() {
	if debugOn() {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	app := cli.App("sshx", "a webrtc based ssh remote tool")
	app.Command("daemon", "launch a sshx daemon", cmdDaemon)
	app.Command("config", "list configure informations", cmdConfig)
	app.Command("connect", "connect to remote host", cmdConnect)
	app.Command("copy", "copy files or directory from/to remote host", cmdCopy)
	app.Command("proxy", "start proxy", cmdProxy)
	app.Command("status", "get status", cmdStatus)
	app.Run(os.Args)

}
