package impl

import (
	"io"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
)

type BaseImpl struct {
	PipeClient net.Conn
	PipeServer net.Conn
	HId        string
	Conn       *net.Conn
	Parent     string
	PId        string
}

func (base *BaseImpl) Init() {
	base.PipeClient, base.PipeServer = net.Pipe()
	if base.Conn != nil {
		logrus.Debug("pipe connection")
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			wg.Done()
			utils.Pipe(base.Conn, &base.PipeClient)
		}()
		wg.Wait()
	}

}

func (base *BaseImpl) PairId() string {
	return base.PId
}

func (base *BaseImpl) SetPairId(id string) {
	base.PId = id
}

func (base *BaseImpl) ParentId() string {
	return base.Parent
}

func (base *BaseImpl) SetParentId(id string) {
	base.Parent = id
}

func (base *BaseImpl) SetConn(conn net.Conn) {
	logrus.Debug("set connection (non-detach)")
	base.Conn = &conn
}

func (base *BaseImpl) Reader() io.Reader {
	return base.PipeServer
}

func (base *BaseImpl) Writer() io.Writer {
	return base.PipeServer
}

func (base *BaseImpl) HostId() string {
	return base.HId
}

func (base *BaseImpl) Close() {
	if base.PipeServer != nil {
		base.PipeServer.Close()
	}
	if base.PipeClient != nil {
		base.PipeClient.Close()
	}
	if base.Conn != nil {
		(*base.Conn).Close()
	}
	logrus.Debug("close base impl")
}
