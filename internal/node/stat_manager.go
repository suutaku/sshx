package node

import (
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

type StatManager struct {
	stats map[string]types.Status
}

func NewStatManager() *StatManager {
	return &StatManager{
		stats: make(map[string]types.Status),
	}
}

func (stm *StatManager) Put(stat types.Status) {
	if stat.PairId == "" {
		logrus.Warn("empty paird id for status: ", stat)
		return
	}
	stm.stats[stat.PairId] = stat
}

func (stm *StatManager) Get() []types.Status {
	ret := make([]types.Status, 0)
	for _, v := range stm.stats {
		ret = append(ret, v)
	}
	return ret
}

func (stm *StatManager) Remove(pid string) {
	delete(stm.stats, pid)
}
