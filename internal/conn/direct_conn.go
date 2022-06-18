package conn

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

type DirectConnection struct {
	BaseConnection
	net.Conn
	CleanChan *chan string
}

func NewDirectConnection(impl impl.Impl, nodeId string, targetId string, poolId types.PoolId, cleanChan *chan string) *DirectConnection {
	ret := &DirectConnection{
		BaseConnection: *NewBaseConnection(impl, nodeId, targetId, poolId),
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
		Id:       dc.poolId.Raw(),
	}
	logrus.Debug("send direct info")
	gob.NewEncoder(conn).Encode(info)
	err = dc.BaseConnection.Dial()
	if err != nil {
		return err
	}
	dc.Exit <- err
	implConn := dc.impl.Conn()
	dc.Conn = conn
	go func() {
		utils.Pipe(&implConn, &dc.Conn)
		*dc.CleanChan <- dc.PoolId().String()
	}()
	return nil
}

func (dc *DirectConnection) Response() error {
	err := dc.BaseConnection.Response()
	if err != nil {
		return err
	}
	implConn := dc.impl.Conn() //connection from dial ssh
	go func() {
		utils.Pipe(&implConn, &dc.Conn)
		*dc.CleanChan <- dc.poolId.String()
	}()

	return nil
}
