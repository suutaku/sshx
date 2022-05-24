package main

import (
	"strings"

	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdSSHFS(cmd *cli.Cmd) {
	cmd.Command("mount", "mount a remote filesystem", cmdMount)
	cmd.Command("unmount", "unmount a remote filesystem", cmdUnmount)
}

func cmdUnmount(cmd *cli.Cmd) {
	cmd.Spec = "PID"
	pidOpt := cmd.StringArg("PID", "", "vnc server pair Id")
	cmd.Action = func() {
		if pidOpt == nil || *pidOpt == "" {
			return
		}
		sender := impl.NewSender(&impl.SSHFS{}, types.OPTION_TYPE_DOWN)
		sender.PairId = []byte(*pidOpt)
		sender.SendDetach()
	}
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
		root, mtp := splitMountPoint(*mtpOpt)
		imp := impl.NewSSHFS(mtp, root, *host, *ident)
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}

		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		_, err = sender.SendDetach()
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Infof("Mount %s %s to %s\n", imp.HostId(), root, mtp)
	}
}
