package conn

import (
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

// manage all supported connection implementations
type ConnectionManager struct {
	css []ConnectionService
	stm *StatManager
}

func NewConnectionManager(enabledService []ConnectionService) *ConnectionManager {
	return &ConnectionManager{
		stm: NewStatManager(),
		css: enabledService,
	}
}

func (cm *ConnectionManager) Start() {
	logrus.Debug("Start connection manager")
	for _, v := range cm.css {
		v.SetStateManager(cm.stm)
		v.Start()
		typeName := ""
		if t := reflect.TypeOf(v); t.Kind() == reflect.Ptr {
			typeName = "*" + t.Elem().Name()
		} else {
			typeName = t.Name()
		}
		logrus.Debug("Start ", typeName)
	}
}

func (cm *ConnectionManager) Stop() {
	for _, v := range cm.css {
		v.Stop()
	}
}

func (cm *ConnectionManager) CreateConnection(sender impl.Sender, sock net.Conn, poolId int64) error {
	errCh := make(chan error, len(cm.css))
	for i := 0; i < len(cm.css); i++ {
		go func(idx int) {
			if cm.css[idx].IsReady() {
				errCh <- cm.css[idx].CreateConnection(sender, sock, poolId)
			}
		}(i)
	}
	count := 0
	for i := 0; i < len(cm.css); i++ {
		err := <-errCh
		if err != nil {
			logrus.Error(err)
			count++
		}
	}
	if count == len(cm.css) {
		return fmt.Errorf("all connection service was not ready")
	}
	return nil
}

func (cm *ConnectionManager) DestroyConnection(sender impl.Sender) error {
	for _, v := range cm.css {
		v.DestroyConnection(sender)
	}
	return nil
}

func (cm *ConnectionManager) AttachConnection(sender impl.Sender, sock net.Conn) error {
	for _, v := range cm.css {
		v.AttachConnection(sender, sock)
	}
	return nil
}

func (cm *ConnectionManager) Status() []types.Status {
	return cm.stm.Stat()
}

type StatManager struct {
	stats    map[string]types.Status
	children map[string][]string
	cpPool   map[string]Connection
	running  bool
	lock     sync.Mutex
}

func NewStatManager() *StatManager {

	return &StatManager{
		stats:    make(map[string]types.Status),
		children: make(map[string][]string),
		cpPool:   make(map[string]Connection),
	}
}

func (stm *StatManager) Stop() {
	stm.running = false
}

func (stm *StatManager) addChild(parent, child string) {
	if stm.children[parent] == nil {
		stm.children[parent] = make([]string, 0)
	}
	stm.children[parent] = append(stm.children[parent], child)
}

func (stm *StatManager) getChildren(parent string) []string {
	return stm.children[parent]
}

func (stm *StatManager) removeParent(parent string) {
	delete(stm.children, parent)
}

func (stm *StatManager) putStat(stat types.Status) {
	if stat.PairId == "" {
		logrus.Warn("empty paird id for status: ", stat)
		return
	}
	if stm.stats[stat.PairId].PairId == stat.PairId {
		return
	}
	stm.stats[stat.PairId] = stat
	logrus.Debug("put status ", stat.PairId)
}

func (stm *StatManager) getStat() []types.Status {
	ret := make([]types.Status, 0)

	for _, v := range stm.stats {
		ret = append(ret, []types.Status{v}...)
	}
	return ret
}

func (stm *StatManager) removeStat(pid string) {
	delete(stm.stats, pid)
	logrus.Debug("remove status for ", pid)
}

func PoolIdFromInt(id int64) string {
	return fmt.Sprintf("conn_%d", id)
}

func (stm *StatManager) Stat() []types.Status {
	return stm.getStat()
}

func (stm *StatManager) RemovePair(id string) {
	stm.lock.Lock()
	defer stm.lock.Unlock()
	children := stm.getChildren(id)
	logrus.Debug("ready to clear children ", children)
	// close children
	for _, v := range children {
		if stm.cpPool[v] != nil {
			stm.cpPool[v].Close()
			delete(stm.cpPool, v)
		}
		stm.removeStat(id)
	}
	// close parent
	if stm.cpPool[id] != nil {
		stm.cpPool[id].Close()
		delete(stm.cpPool, id)
	}
	stm.removeStat(id)
	stm.removeParent(id)
}

func (stm *StatManager) AddPair(pair Connection) error {
	stm.lock.Lock()
	defer stm.lock.Unlock()
	if pair == nil {
		return fmt.Errorf("pair was empty")
	}
	if stm.cpPool[pair.PoolIdStr()] != nil {
		return fmt.Errorf("pair already exist")
	}
	stm.cpPool[pair.PoolIdStr()] = pair
	stat := types.Status{
		PairId:    pair.PoolIdStr(),
		TargetId:  pair.TargetId(),
		ImplType:  pair.GetImpl().Code(),
		StartTime: time.Now(),
	}

	if pair.GetImpl().ParentId() != "" {
		logrus.Debug("add child ", pair.PoolIdStr(), " to ", pair.GetImpl().ParentId())
		stat.ParentPairId = pair.GetImpl().ParentId()
		stm.addChild(pair.GetImpl().ParentId(), pair.PoolIdStr())
	}
	stm.putStat(stat)
	logrus.Debug("put pair on stat ", impl.GetImplName(pair.GetImpl().Code()), " with pair id ", stat.PairId)
	return nil
}

func (stm *StatManager) GetPair(id string) Connection {
	return stm.cpPool[id]
}
