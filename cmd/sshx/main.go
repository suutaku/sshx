package main

import (
	"os"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
)

var defaultHomePath = "/etc/sshx"

func main() {
	if utils.DebugOn() {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	app := cli.App("sshx", "a webrtc based ssh remote toolbox")
	app.Command("daemon", "launch a sshx daemon", cmdDaemon)
	app.Command("conf", "list configure informations", cmdConfig)
	app.Command("conn", "connect to remote host", cmdConnect)
	app.Command("cpyid", "copy public key to server", cmdCopyId)
	app.Command("scp", "copy files or directory from/to remote host", cmdCopy)
	app.Command("proxy", "start proxy", cmdProxy)
	app.Command("stat", "get status", cmdStatus)
	app.Command("fs", "sshfs filesystem", cmdSSHFS)
	app.Command("vnc", "vnc service", cmdVNCService)
	app.Command("msg", "a message console", cmdMessage)
	app.Command("trans", "transfer a file", cmdTransfer)
	app.Run(os.Args)

}
