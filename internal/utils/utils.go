package utils

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

const customDomain = "vnc.sshx.wz"

var alias = fmt.Sprintf("127.0.0.1    %s\n", customDomain)

const macHostFilePath = "/private/etc/hosts"
const linuxHostFilePath = "/etc/hosts"

func FixDNSAlias() {
	confPath := ""
	osname := runtime.GOOS
	switch osname {
	case "windows":
		logrus.Println("Platform: Windows")
	case "darwin":
		logrus.Println("Platform: MAC OSX")
		confPath = macHostFilePath
	case "linux":
		logrus.Println("Platform: Linux")
		confPath = linuxHostFilePath
	default:
		logrus.Printf("Platform %s was not supported yet.\n", osname)
	}
	file, err := os.OpenFile(confPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		logrus.Error(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), customDomain) {
			logrus.Info("alias alredy exist in ", confPath)
			return
		}
	}
	_, err = file.WriteString(alias)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Info("fix DNS alias successfully")
}
