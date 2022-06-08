package node

import (
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

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
