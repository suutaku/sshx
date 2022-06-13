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
	UIOpened   bool
	isRuning   bool
	sendChan   chan Message
	recvChan   chan Message
	attachConn net.Conn
}

func NewMessager(hostId string) *Messager {
	return &Messager{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
		sendChan: make(chan Message, 1024),
		recvChan: make(chan Message, 1024),
	}
}

func (m *Messager) Code() int32 {
	return types.APP_TYPE_MESSAGER
}

func (m *Messager) OpenUI() error {
	m.UIOpened = true
	return nil
}

func (m *Messager) serveRecv() {
	if m.recvChan == nil {
		m.recvChan = make(chan Message, 1024)
	}
	for {
		var msg Message
		logrus.Debug("waiting message")
		err := gob.NewDecoder(m.attachConn).Decode(&msg)
		logrus.Debug("waiting message ok")
		if err != nil {
			logrus.Error(err)
			m.Close()
			return
		}
		logrus.Debug("message come ", string(msg.Payload))
		if !m.UIOpened {
			notify.Notify("sshx", "message", string(msg.Payload), "")
		}
		select {
		case m.recvChan <- msg:
			logrus.Debug("push message to recv chan")
		default:
			<-m.recvChan
			logrus.Warn("drop a message")
		}
	}
}

func (m *Messager) serveSend() {
	if m.sendChan == nil {
		m.sendChan = make(chan Message, 1024)
	}
	for {
		msg := <-m.sendChan
		err := gob.NewEncoder(m.attachConn).Encode(msg)
		if err != nil {
			logrus.Error(err)
			m.Close()
			return
		}
	}
}

func (m *Messager) Response() error {
	// a naive test code
	m.isRuning = true
	c, s := net.Pipe()
	m.attachConn = c
	m.BaseImpl.SetConn(s)
	go m.serveRecv()
	go m.serveSend()
	return nil
}

func (m *Messager) Attach(conn net.Conn) error {
	m.UIOpened = true
	go func() {
		for {
			msg, ok := <-m.recvChan
			if !ok {
				conn.Close()
				m.Close()
				return
			}
			err := gob.NewEncoder(conn).Encode(msg)
			if err != nil {
				conn.Close()
				m.Close()
				return
			}
		}
	}()
	go func() {
		for {
			var msg Message
			err := gob.NewDecoder(conn).Decode(&msg)
			if err != nil {
				conn.Close()
				m.Close()
				return
			}
			m.sendChan <- msg
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
			err = gob.NewEncoder(conn).Encode(outMsg)
			if err != nil {
				logrus.Debug(err)
				conn.Close()
				return
			}
			logrus.Debug("send to remote")
		}

	}()

	// recieve
	for m.isRuning {
		var inMsg Message
		err := gob.NewDecoder(conn).Decode(&inMsg)
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
	if m.conn != nil {
		(*m.conn).Close()
	}
	if m.attachConn != nil {
		m.attachConn.Close()
	}
	safeClose(m.recvChan)
	safeClose(m.sendChan)
	m.BaseImpl.Close()
}

func safeClose(ch chan Message) (justClosed bool) {
	defer func() {
		if recover() != nil {
			// The return result can be altered
			// in a defer function call.
			justClosed = false
		}
	}()
	// assume ch != nil here.
	close(ch)   // panic if ch is closed
	return true // <=> justClosed = true; return
}
