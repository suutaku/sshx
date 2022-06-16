package conn

import (
	"encoding/gob"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

// a connection service interface
type ConnectionService interface {
	Start() error
	SetStateManager(*StatManager) error
	CreateConnection(impl.Sender, net.Conn) error
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
	cpPool    map[string]Connection
	CleanChan chan string
	id        string
}

func NewBaseConnectionService(id string) *BaseConnectionService {
	return &BaseConnectionService{
		CleanChan: make(chan string, 10),
		cpPool:    make(map[string]Connection),
		id:        id,
	}
}

func (base *BaseConnectionService) Id() string {
	return base.id
}

func (base *BaseConnectionService) RemovePair(id string) {
	children := base.stm.GetChildren(id)
	logrus.Debug("ready to clear children ", children)
	// close children
	for _, v := range children {
		if base.cpPool[v] != nil {
			base.cpPool[v].Close()
			delete(base.cpPool, v)
		}
		base.stm.Remove(id)
	}
	// close parent
	if base.cpPool[id] != nil {
		base.cpPool[id].Close()
		delete(base.cpPool, id)
	}
	base.stm.Remove(id)
	base.stm.RemoveParent(id)
}
func (base *BaseConnectionService) AddPair(pair Connection) {
	// if node.cpPool[id] != nil {
	// 	logrus.Warn("recover connection pair ", id)
	// 	node.RemovePair(id)
	// }
	if pair == nil {
		return
	}
	base.cpPool[pair.PoolIdStr()] = pair
	stat := types.Status{
		PairId:    pair.PoolIdStr(),
		TargetId:  pair.TargetId(),
		ImplType:  pair.GetImpl().Code(),
		StartTime: time.Now(),
	}

	if pair.GetImpl().ParentId() != "" {
		logrus.Debug("add child ", pair.PoolIdStr(), " to ", pair.GetImpl().ParentId())
		stat.ParentPairId = pair.GetImpl().ParentId()
		base.stm.AddChild(pair.GetImpl().ParentId(), pair.PoolIdStr())
	}
	base.stm.Put(stat)

}

func (base *BaseConnectionService) GetPair(id string) Connection {
	return base.cpPool[id]
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

func (base *BaseConnectionService) CreateConnection(impl.Sender, net.Conn) error {
	logrus.Warn("CreateConnection not implemeted")
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
