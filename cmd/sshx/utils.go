package main

import (
	"errors"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

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
