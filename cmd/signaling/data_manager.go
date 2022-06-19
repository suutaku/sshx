package main

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	LIFE_TIME_IN_SECOND = 15
	MAX_BUFFER_NUMBER   = 64
)

type DManager struct {
	datas map[string]chan types.SignalingInfo
	mu    sync.Mutex
	alive map[string]int
}

func NewDManager() *DManager {
	return &DManager{
		datas: make(map[string]chan types.SignalingInfo),
		alive: make(map[string]int),
	}
}

func (dm *DManager) Get(id string) chan types.SignalingInfo {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.datas[id]
}

func (dm *DManager) Clean(id string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if dm.datas[id] != nil {
		close(dm.datas[id])
	}
	delete(dm.datas, id)
	delete(dm.alive, id)
}

func (dm *DManager) resetAlive(id string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.alive[id] = LIFE_TIME_IN_SECOND
}

func (dm *DManager) Set(id string, info types.SignalingInfo) {
	if dm.datas[id] == nil {
		dm.mu.Lock()
		dm.datas[id] = make(chan types.SignalingInfo, MAX_BUFFER_NUMBER)
		dm.mu.Unlock()
		dm.resetAlive(id)
		go func(dmc *DManager) {
			logrus.Debug("create watch dog for ", id)
			for dmc.alive[id] > 0 {
				time.Sleep(time.Second)
				dmc.mu.Lock()
				dmc.alive[id]--
				dmc.mu.Unlock()
			}
			logrus.Debug("execute watch dog for ", id)
			dm.Clean(id)
		}(dm)
	}
	select {
	case dm.datas[id] <- info:
		dm.resetAlive(id)
	default:
	}
}
