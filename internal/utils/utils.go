package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

func PipeWR(reader1, reader2 io.Reader, writer1, writer2 io.Writer) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(writer1, reader2)
	}()

	go func() {
		defer wg.Done()
		io.Copy(writer2, reader1)
	}()
	wg.Wait()
	logrus.Info("pipe closed")
}

func Pipe(con1 *net.Conn, con2 *net.Conn) {
	logrus.Debug("do pipe connection")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(*con1, *con2)
		if err != nil {
			logrus.Error(err)
		}
		if con1 != nil {
			(*con1).Close()
		}
		if con2 != nil {
			(*con2).Close()
		}

		logrus.Debug("io copy 1 closed")
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(*con2, *con1)
		if err != nil {
			logrus.Error(err)
		}
		if con1 != nil {
			(*con1).Close()
		}
		if con2 != nil {
			(*con2).Close()
		}
		logrus.Debug("io copy 2 closed")
	}()
	wg.Wait()
	logrus.Info("pipe closed")
}

func ToNetConn(wsconn *websocket.Conn) *net.Conn {
	return &[]net.Conn{
		wsconn,
	}[0]
}

func HashString(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}

func DebugOn() bool {
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

func GetSSHXHome() string {
	home := os.Getenv("SSHX_HOME")
	if home == "" {
		home = "/etc/sshx"
	}
	return home
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func MakeRandomStr(digit uint32) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// 乱数を生成
	b := make([]byte, digit)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("unexpected error")
	}

	// letters からランダムに取り出して文字列を生成
	var result string
	for _, v := range b {
		// index が letters の長さに収まるように調整
		result += string(letters[int(v)%len(letters)])
	}
	return result, nil
}
