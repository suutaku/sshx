package main

import (
	"strings"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/impl"
)

func cmdSSHFS(cmd *cli.Cmd) {
	cmd.Command("mount", "mount a remote filesystem", cmdMount)
	cmd.Command("unmount", "unmount a remote filesystem", cmdUnmount)
}

func cmdUnmount(cmd *cli.Cmd) {

}

func splitMountPoint(mtopt string) (root, mt string) {
	sp := strings.Split(mtopt, ":")
	if len(sp) < 2 {
		return
	}
	root = sp[0]
	mt = sp[1]
	return
}

func cmdMount(cmd *cli.Cmd) {
	cmd.Spec = "[-i] HOST MOUNTOPTION"
	host := cmd.StringArg("HOST", "", "moumt root path")
	mtpOpt := cmd.StringArg("MOUNTOPTION", "", "moumt option with [root]:[mount point]")
	ident := cmd.StringOpt("i identification", "", "a private path, default empty for ~/.ssh/id_rsa")
	cmd.Action = func() {
		if host == nil || *(host) == "" {
			return
		}
		cm := conf.NewConfManager(getRootPath())
		dialer := impl.NewSfsImpl()
		param := impl.ImplParam{
			Config: *cm.Conf,
		}
		dialer.Init(param)
		err := dialer.DecodeAddress(*host)
		if err != nil {
			logrus.Error(err)
			return
		}
		root, mtp := splitMountPoint(*mtpOpt)
		dialer.SetMountPoint(mtp)
		dialer.SetRoot(root)
		dialer.PrivateKeyOption(*ident)

		err = dialer.Dial()
		if err != nil {
			logrus.Info(err)
			return
		}

	}
}
