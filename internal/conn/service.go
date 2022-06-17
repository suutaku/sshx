package conn

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

// a connection service interface
type ConnectionService interface {
	Start() error
	SetStateManager(*StatManager) error
	CreateConnection(impl.Sender, net.Conn, types.PoolId) error
	DestroyConnection(impl.Sender) error
	AttachConnection(impl.Sender, net.Conn) error
	IsReady() bool
	Stop()
	GetPair(id string) Connection
	WatchPairs()
	Id() string
}

type BaseConnectionService struct {
	stm       *StatManager
	isReady   bool
	running   bool
	CleanChan chan string
	id        string
}

func NewBaseConnectionService(id string) *BaseConnectionService {
	return &BaseConnectionService{
		CleanChan: make(chan string, 10),
		id:        id,
	}
}

func (base *BaseConnectionService) Id() string {
	return base.id
}

func (base *BaseConnectionService) RemovePair(id string) {
	base.stm.RemovePair(id)
}
func (base *BaseConnectionService) AddPair(pair Connection) error {
	return base.stm.AddPair(pair)
}

func (base *BaseConnectionService) GetPair(id string) Connection {
	return base.stm.GetPair(id)
}

func (base *BaseConnectionService) WatchPairs() {
	for base.running {
		pairId := <-base.CleanChan
		base.RemovePair(pairId)
		logrus.Debug("clean request from clean channel ", pairId)
	}
}

func (base *BaseConnectionService) Start() error {
	base.running = true
	base.isReady = true
	go base.WatchPairs()
	return nil
}

func (base *BaseConnectionService) SetStateManager(stm *StatManager) error {
	base.stm = stm
	return nil
}

func (base *BaseConnectionService) CreateConnection(sender impl.Sender, conn net.Conn, poolId types.PoolId) error {

	if base.GetPair(poolId.String()) != nil {

		return fmt.Errorf("connection already exist for %s", poolId.String())
	}
	return nil
}

func (base *BaseConnectionService) DestroyConnection(tmp impl.Sender) error {
	pair := base.GetPair(string(tmp.PairId))
	if pair == nil {
		return fmt.Errorf("cannot get pair for %s", string(tmp.PairId))
	}
	if pair.GetImpl().Code() == tmp.GetAppCode() {
		base.RemovePair(string(tmp.PairId))
	}
	return nil
}

func (base *BaseConnectionService) AttachConnection(sender impl.Sender, sock net.Conn) error {
	pair := base.GetPair(string(sender.PairId))
	if pair == nil {
		return fmt.Errorf("cannot attach impl with id: %s", string(sender.PairId))

	}
	// should assign host id and return
	retSender := impl.NewSender(pair.GetImpl(), types.OPTION_TYPE_ATTACH)
	err := gob.NewEncoder(sock).Encode(retSender)
	if err != nil {
		return err
	}
	pair.GetImpl().Attach(sock)
	return nil
}

func (base *BaseConnectionService) IsReady() bool {
	return base.isReady
}

func (base *BaseConnectionService) Stop() {
	base.running = false
	base.isReady = false
}
