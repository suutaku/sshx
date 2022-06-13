package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/martinlindhe/notify"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/term"
)

type Message struct {
	Type    int32
	Payload []byte
}

type Messager struct {
	BaseImpl
	UIOpened bool
	isRuning bool
}

func NewMessager(hostId string) *Messager {
	return &Messager{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
	}
}

func (m *Messager) Code() int32 {
	return types.APP_TYPE_MESSAGER
}

func (m *Messager) OpenUI() error {
	m.UIOpened = true
	return nil
}

func (m *Messager) Response() error {
	// a naive test code
	m.isRuning = true
	go func() {
		c, s := net.Pipe()
		m.Conn = &s
		dec := gob.NewDecoder(c)
		for m.isRuning {
			var msg Message
			logrus.Debug("waiting message")
			err := dec.Decode(&msg)
			logrus.Debug("waiting message ok")
			if err != nil {
				m.Close()
				return
			}
			logrus.Debug("message come ", string(msg.Payload))
			if !m.UIOpened {
				notify.Notify("sshx", "message", string(msg.Payload), "")
			}
		}
	}()
	return nil
}

func (m *Messager) OpenChatConsole(conn io.ReadWriteCloser) {
	if conn == nil {
		return
	}
	if !term.IsTerminal(0) || !term.IsTerminal(1) {
		return
	}
	oldState, err := term.MakeRaw(0)
	if err != nil {
		return
	}
	defer term.Restore(0, oldState)
	m.UIOpened = true
	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	term := term.NewTerminal(screen, "")
	term.SetPrompt(string(term.Escape.Red) + "> " + string(term.Escape.Reset))

	rePrefix := string(term.Escape.Cyan) + m.HId + ":" + string(term.Escape.Reset)
	m.isRuning = true
	go func() {
		// send
		enc := gob.NewEncoder(conn)
		for m.isRuning {
			line, err := term.ReadLine()
			if err != nil {
				logrus.Debug(err)
				conn.Close()
				return
			}
			var outMsg = Message{
				Payload: []byte(line),
			}
			err = enc.Encode(&outMsg)
			if err != nil {
				conn.Close()
				return
			}
			logrus.Debug("send to remote")
		}

	}()

	// recieve
	enc := gob.NewDecoder(conn)
	for m.isRuning {
		var inMsg Message
		err := enc.Decode(&inMsg)
		if err != nil {
			logrus.Debug(err)
			conn.Close()
			return
		}
		if err != nil {
			logrus.Debug(err)
			conn.Close()
			return
		}
		fmt.Fprintln(term, rePrefix, string(inMsg.Payload))
	}
}

func (m *Messager) Close() {
	m.isRuning = false
	m.BaseImpl.Close()
}
