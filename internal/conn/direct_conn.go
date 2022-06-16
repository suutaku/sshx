package conn

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/impl"
)

type DirectConnection struct {
	BaseConnection
	net.Conn
	CleanChan *chan string
}

func NewDirectConnection(impl impl.Impl, nodeId string, targetId string, cleanChan *chan string) *DirectConnection {
	ret := &DirectConnection{
		BaseConnection: *NewBaseConnection(impl, nodeId, targetId),
		CleanChan:      cleanChan,
	}
	ret.CleanChan = cleanChan
	return ret
}

func (dc *DirectConnection) Close() {
	dc.BaseConnection.Close()
	dc.Conn.Close()
}

func (dc *DirectConnection) Dial() error {
	logrus.Debug("dial ", dc.TargetId(), " directly")
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", dc.TargetId(), directPort))
	if err != nil {
		return err
	}
	info := DirectInfo{
		ImplCode: dc.impl.Code(),
		HostId:   dc.nodeId,
	}
	logrus.Debug("send DirectInfo")
	gob.NewEncoder(conn).Encode(info)
	logrus.Debug("send DirectInfo ok")
	go dc.impl.Dial()
	logrus.Debug("send exit signal")
	dc.Exit <- err
	logrus.Debug("send exit signal ok")
	implConn := dc.impl.Conn()
	dc.Conn = conn
	logrus.Debug("pip on")
	go utils.Pipe(&implConn, &dc.Conn)
	return nil
}

func (dc *DirectConnection) Response() error {
	dc.impl.Response()
	implConn := dc.impl.Conn() //connection from dial ssh
	go utils.Pipe(&implConn, &dc.Conn)

	return nil
}
