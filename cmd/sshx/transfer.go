package main

import (
	cli "github.com/jawher/mow.cli"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func cmdUpload(cmd *cli.Cmd) {
	cmd.Spec = "TARGETID FILEPATH"
	hostId := cmd.StringArg("TARGETID", "", "target device id")
	filePath := cmd.StringArg("FILEPATH", "", "path of file to upload")
	cmd.Action = func() {

		if hostId == nil || *hostId == "" {

		}
		if filePath == nil || *filePath == "" {
			logrus.Error("please set a file path")
		}

		imp := impl.NewTransfer(*hostId, *filePath, true)
		err := imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := impl.NewSender(imp, types.OPTION_TYPE_UP)
		conn, err := sender.Send()
		if err != nil {
			logrus.Error(err)
			return
		}
		imp.SetConn(conn)
		imp.ShowQR()
		imp.Close()
	}

}
func cmdDownload(cmd *cli.Cmd) {
	cmd.Spec = "TARGETID FILEPATH"
	hostId := cmd.StringArg("TARGETID", "", "target device id")
	filePath := cmd.StringArg("FILEPATH", "", "path of file to download")
	cmd.Action = func() {
		if hostId == nil || *hostId == "" {

		}
		if filePath == nil || *filePath == "" {
			logrus.Error("please set a file path")
		}
	}
}

func cmdTransfer(cmd *cli.Cmd) {
	cmd.Command("upload", "upload file to target device", cmdUpload)
	cmd.Command("stop", "download file from target device", cmdDownload)
}
