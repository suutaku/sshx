package conn

import (
	"encoding/gob"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	CONNECTION_DRECT_IN = iota
	CONNECTION_DRECT_OUT
)

type Connection interface {
	Close()
	GetImpl() impl.Impl
	PoolId() *types.PoolId
	ResetPoolId(id types.PoolId)
	ResponseTCP(resp impl.Sender)
	TargetId() string
	Dial() error
	Response() error
	Direction() int32
	IsReady() bool
	Ready()
	Name() string
}

type BaseConnection struct {
	impl     impl.Impl
	nodeId   string
	targetId string
	poolId   types.PoolId
	Exit     chan error
	Direct   int32
	ready    bool
}

func NewBaseConnection(impl impl.Impl, nodeId, targetId string, poolId types.PoolId, direct, implc int32) *BaseConnection {
	impl.Init()
	ret := &BaseConnection{
		Exit:     make(chan error, 10),
		nodeId:   nodeId,
		targetId: targetId,
		poolId:   poolId,
		impl:     impl,
		Direct:   direct,
	}
	if ret.PoolId().Raw() == 0 {
		ret.poolId = *types.NewPoolId(time.Now().UnixNano(), implc)
	}
	return ret
}

func (bc *BaseConnection) Ready() {
	bc.ready = true
}

func (bc *BaseConnection) IsReady() bool {
	return bc.ready
}

func (bc *BaseConnection) Direction() int32 {
	return bc.Direct
}

func (bc *BaseConnection) Close() {
	logrus.Debug("close pair")
	if bc.impl != nil {
		bc.impl.Close()
	}
}

func (bc *BaseConnection) PoolId() *types.PoolId {
	return &bc.poolId
}
func (bc *BaseConnection) GetImpl() impl.Impl {
	return bc.impl
}

func (bc *BaseConnection) ResetPoolId(id types.PoolId) {
	logrus.Debug("reset pool id from ", bc.poolId, " to ", id)
	bc.poolId = id
}

func (bc *BaseConnection) TargetId() string {
	return bc.targetId
}

func (bc *BaseConnection) Dial() error {
	bc.impl.Dial()
	return nil
}
func (bc *BaseConnection) Response() error {
	logrus.Debug("base connection response")
	bc.impl.Response()
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
