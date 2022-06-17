package conn

import (
	"encoding/gob"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
)

const (
	CONNECTION_DRECT_IN = iota
	CONNECTION_DRECT_OUT
)

type Connection interface {
	Close()
	GetImpl() impl.Impl
	PoolIdStr() string
	PoolId() int64
	ResetPoolId(id int64)
	ResponseTCP(resp impl.Sender)
	TargetId() string
	Dial() error
	Response() error
}

type BaseConnection struct {
	impl      impl.Impl
	nodeId    string
	targetId  string
	poolId    int64
	Exit      chan error
	Derection int32
}

func NewBaseConnection(impl impl.Impl, nodeId, targetId string, poolId int64) *BaseConnection {
	impl.Init()
	ret := &BaseConnection{
		Exit:     make(chan error, 10),
		nodeId:   nodeId,
		targetId: targetId,
		poolId:   poolId,
		impl:     impl,
	}
	if ret.poolId == 0 {
		ret.poolId = time.Now().UnixNano()
	}
	return ret
}

func (bc *BaseConnection) Close() {
	logrus.Debug("close pair")
	if bc.impl != nil {
		bc.impl.Close()
	}
}

func (bc *BaseConnection) PoolIdStr() string {
	return PoolIdFromInt(bc.poolId)
}
func (bc *BaseConnection) PoolId() int64 {
	return bc.poolId
}
func (bc *BaseConnection) GetImpl() impl.Impl {
	return bc.impl
}

func (bc *BaseConnection) ResetPoolId(id int64) {
	logrus.Debug("reset pool id from ", bc.poolId, " to ", id)
	bc.poolId = id
}

func (bc *BaseConnection) TargetId() string {
	return bc.targetId
}

func (bc *BaseConnection) Dial() error {
	bc.Derection = CONNECTION_DRECT_OUT
	go bc.impl.Dial()
	return nil
}
func (bc *BaseConnection) Response() error {
	bc.Derection = CONNECTION_DRECT_IN
	go bc.impl.Response()
	return nil
}

func (bc *BaseConnection) ResponseTCP(resp impl.Sender) {
	logrus.Debug("waiting pair signal")
	err := <-bc.Exit
	logrus.Debug("Response TCP")
	if err != nil {
		logrus.Error(err)
		resp.Status = -1
	}
	err = gob.NewEncoder(bc.impl.Writer()).Encode(resp)
	if err != nil {
		logrus.Error(err)
		return
	}
}
