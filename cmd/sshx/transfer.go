package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdUpload(cmd *cli.Cmd) {
	cmd.Spec = "[-t] [-f] [-q]"
	hostId := cmd.StringOpt("t target", "", "target device id")
	filePath := cmd.StringOpt("f file", "", "path of file to upload")
	showQR := cmd.BoolOpt("q qrcode", false, "show QR code (upload from or download to mobile device)")
	cmd.Action = func() {

		if hostId == nil || *hostId == "" {
			*hostId = "127.0.0.1"
		}

		imp := impl.NewTransferService(*hostId, *filePath, true, *showQR)
		imp.Init()
		imp.NoNeedConnect()
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		conn, err := sender.SendDetach()
		if err != nil {
			logrus.Error(err)
			return
		}
		imp.PId = string(sender.PairId)
		imp.SetConn(conn)
		err = imp.Start()
		if err != nil {
			logrus.Error(err)
		}
		imp.Close()
	}

}
func cmdDownload(cmd *cli.Cmd) {
	cmd.Spec = "[-t] [-f] [-q]"
	hostId := cmd.StringOpt("t target", "", "target device id")
	filePath := cmd.StringOpt("f file", "", "path of file to download")
	showQR := cmd.BoolOpt("q qrcode", false, "show QR code (upload from or download to mobile device)")
	cmd.Action = func() {
		if hostId == nil || *hostId == "" {
			*hostId = "127.0.0.1"
		}

		imp := impl.NewTransferService(*hostId, *filePath, false, *showQR)
		if imp == nil {
			logrus.Error("Please input a remote file path when using download command")
			return
		}
		imp.Init()
		imp.NoNeedConnect()
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		conn, err := sender.SendDetach()
		if err != nil {
			logrus.Error(err)
			return
		}
		imp.PId = string(sender.PairId)
		imp.SetConn(conn)
		err = imp.Start()
		if err != nil {
			logrus.Error(err)
		}
		imp.Close()
	}
}

func cmdTransfer(cmd *cli.Cmd) {
	cmd.Command("upload", "upload file to target device", cmdUpload)
	cmd.Command("download", "download file from target device", cmdDownload)
}
