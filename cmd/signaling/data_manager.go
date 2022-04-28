package main

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	LIFE_TIME         = time.Second * 15
	MAX_BUFFER_NUMBER = 64
)

type DManager struct {
	datas map[string]chan types.SignalingInfo
	mu    sync.Mutex
}

func NewDManager() *DManager {
	return &DManager{
		datas: make(map[string]chan types.SignalingInfo),
	}
}

func (dm *DManager) Get(id string) chan types.SignalingInfo {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.datas[id]
}

func (dm *DManager) Set(id string, info types.SignalingInfo) {
	if dm.datas[id] == nil {
		dm.mu.Lock()
		defer dm.mu.Unlock()
		dm.datas[id] = make(chan types.SignalingInfo, MAX_BUFFER_NUMBER)
		go func() {
			logrus.Debug("create watch dog for ", id)
			time.Sleep(LIFE_TIME)
			logrus.Debug("info got timeout for ", id)
			dm.mu.Lock()
			defer dm.mu.Unlock()
			close(dm.datas[id])
			delete(dm.datas, id)
		}()
	}
	select {
	case dm.datas[id] <- info:
	default:
	}
}
