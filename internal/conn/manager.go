package conn

import (
	"fmt"
	"net"
	"reflect"

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

func (cm *ConnectionManager) CreateConnection(sender impl.Sender, sock net.Conn) error {
	for i := 0; i < len(cm.css); i++ {
		if cm.css[i].IsReady() {
			return cm.css[i].CreateConnection(sender, sock)
		}
	}
	return fmt.Errorf("all connection service was not ready")
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
	return cm.stm.Get()
}

type StatManager struct {
	stats    map[string]types.Status
	children map[string][]string
	running  bool
}

func NewStatManager() *StatManager {

	return &StatManager{
		stats:    make(map[string]types.Status),
		children: make(map[string][]string),
	}
}

func (stm *StatManager) Stop() {
	stm.running = false
}

func (stm *StatManager) AddChild(parent, child string) {
	if stm.children[parent] == nil {
		stm.children[parent] = make([]string, 0)
	}
	stm.children[parent] = append(stm.children[parent], child)
}

func (stm *StatManager) GetChildren(parent string) []string {
	return stm.children[parent]
}

func (stm *StatManager) RemoveParent(parent string) {
	delete(stm.children, parent)
}

func (stm *StatManager) Put(stat types.Status) {
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

func (stm *StatManager) Get() []types.Status {
	ret := make([]types.Status, 0)

	for _, v := range stm.stats {
		ret = append(ret, []types.Status{v}...)
	}
	return ret
}

func (stm *StatManager) Remove(pid string) {
	delete(stm.stats, pid)
	logrus.Debug("remove status for ", pid)
}
