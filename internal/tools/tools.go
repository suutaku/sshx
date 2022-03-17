package tools

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
)

var defaultPort = "22"

const (
	ADDR_TYPE_IPV4 = iota
	ADDR_TYPE_IPV6
	ADDR_TYPE_DOMAIN
	ADDR_TYPE_ID
)

func AddrType(addrStr string) int {
	addr := net.ParseIP(addrStr)
	if addr != nil {
		return ADDR_TYPE_IPV4
	}
	if strings.Contains(addrStr, ".") {
		return ADDR_TYPE_DOMAIN
	}

	return ADDR_TYPE_ID

}

func GetParam(addrStr string) (userName, addr, port string, err error) {
	sps := strings.Split(addrStr, "@")
	if len(sps) < 2 {
		err = errors.New("bad address format " + addrStr)
		return
	}
	userName = sps[0]
	sps = strings.Split(sps[1], ":")
	if len(sps) < 2 {
		addr = sps[0]
		port = defaultPort
	} else {
		addr = sps[0]
		port = sps[1]
	}

	return
}

func CreateTmpListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(1000)+10000)
}
