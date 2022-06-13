package impl

import (
	"io"
	"net"

	"github.com/sirupsen/logrus"
)

type BaseImpl struct {
	HId    string
	Conn   *net.Conn
	Parent string
	PId    string
}

func (base *BaseImpl) Preper() error {
	return nil
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
	return *(base.Conn)
}

func (base *BaseImpl) Writer() io.Writer {
	return (*base.Conn)
}

func (base *BaseImpl) ReadWriteCloser() io.ReadWriteCloser {
	return (*base.Conn)
}

func (base *BaseImpl) HostId() string {
	return base.HId
}

func (base *BaseImpl) Close() {
	if base.Conn != nil {
		logrus.Debug("close Conn")
		(*base.Conn).Close()
	}
	logrus.Debug("close base impl")
}

// Response of remote device call
func (base *BaseImpl) Response() error {
	return nil
}

// Call remote device
func (base *BaseImpl) Dial() error {
	return nil
}
