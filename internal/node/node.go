package node

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/types"
)

type Node struct {
	ConfManager *conf.ConfManager
	sigPull     chan *types.SignalingInfo
	sigPush     chan *types.SignalingInfo
	cpPool      map[string]*ConnectionPair
	stm         *StatManager
	running     bool
	CleanChan   chan string
}

func NewNode(home string) *Node {
	return &Node{
		sigPull:     make(chan *types.SignalingInfo, 128),
		sigPush:     make(chan *types.SignalingInfo, 128),
		ConfManager: conf.NewConfManager(home),
		cpPool:      make(map[string]*ConnectionPair),
		stm:         NewStatManager(),
		CleanChan:   make(chan string, 10),
	}
}

func (node *Node) WatchPairs() {
	for node.running {
		pairId := <-node.CleanChan
		node.RemovePair(pairId)
		logrus.Debug("clean request from clean channel ", pairId)
	}
}

func (node *Node) Start() {
	node.running = true
	go node.ServeSignaling()
	go node.WatchPairs()

	node.ServeTCP()
}

func (node *Node) RemovePair(id string) {
	children := node.stm.GetChildren(id)
	logrus.Debug("ready to clear children ", children)
	// close children
	for _, v := range children {
		if node.cpPool[v] != nil {
			node.cpPool[v].Close()
			delete(node.cpPool, v)
		}
		node.stm.Remove(id)
	}
	// close parent
	if node.cpPool[id] != nil {
		node.cpPool[id].Close()
		delete(node.cpPool, id)
	}
	node.stm.Remove(id)
	node.stm.RemoveParent(id)
}
func (node *Node) AddPair(pair *ConnectionPair) {
	// if node.cpPool[id] != nil {
	// 	logrus.Warn("recover connection pair ", id)
	// 	node.RemovePair(id)
	// }
	if pair == nil {
		return
	}
	node.cpPool[pair.PoolIdStr()] = pair
	stat := types.Status{
		PairId:    pair.PoolIdStr(),
		TargetId:  pair.targetId,
		ImplType:  pair.impl.Code(),
		StartTime: time.Now(),
	}

	if pair.impl.ParentId() != "" {
		logrus.Debug("add child ", pair.PoolIdStr(), " to ", pair.impl.ParentId())
		stat.ParentPairId = pair.impl.ParentId()
		node.stm.AddChild(pair.impl.ParentId(), pair.PoolIdStr())
	}
	node.stm.Put(stat)

}

func (node *Node) GetPair(id string) *ConnectionPair {
	return node.cpPool[id]
}

func (node *Node) Stop() {
	node.running = false
	node.stm.Stop()
}
