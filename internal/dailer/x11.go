package dailer

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

type x11request struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

func (dal *Dailer) X11Request() {
	// x11-req Payload
	payload := x11request{
		SingleConnection: false,
		AuthProtocol:     string("MIT-MAGIC-COOKIE-1"),
		AuthCookie:       string("d92c30482cc3d2de61888961deb74c08"),
		ScreenNumber:     uint32(0),
	}

	// NOTE:
	// send x11-req Request
	ok, err := dal.session.SendRequest("x11-req", true, ssh.Marshal(payload))
	if err == nil && !ok {
		fmt.Println(errors.New("ssh: x11-req failed"))
		return
	}
	x11channels := dal.client.HandleChannelOpen("x11")

	go func() {
		for ch := range x11channels {
			channel, _, err := ch.Accept()
			if err != nil {
				continue
			}

			go dal.forwardX11Socket(channel)
		}
	}()
}

func (dal *Dailer) forwardX11Socket(channel ssh.Channel) {
	conn, err := net.Dial("unix", os.Getenv("DISPLAY"))
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(conn, channel)
		conn.(*net.UnixConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(channel, conn)
		channel.CloseWrite()
		wg.Done()
	}()

	wg.Wait()
	conn.Close()
	channel.Close()
}
