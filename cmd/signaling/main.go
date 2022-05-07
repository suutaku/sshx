package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
)

func main() {
	port := os.Getenv("SSHX_SIGNALING_PORT")
	if port == "" {
		port = "11095"
	}

	server := NewServer(port)

	if utils.DebugOn() {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	server.Start()
}
