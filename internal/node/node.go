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
}

func NewNode(home string) *Node {
	return &Node{
		sigPull:     make(chan *types.SignalingInfo, 128),
		sigPush:     make(chan *types.SignalingInfo, 128),
		ConfManager: conf.NewConfManager(home),
		cpPool:      make(map[string]*ConnectionPair),
		stm:         NewStatManager(),
	}
}

func (node *Node) Start() {
	node.running = true
	go node.ServeSignaling()
	go node.stm.Start()
	node.ServeTCP()
}

func (node *Node) RemovePair(id string) {
	if node.cpPool[id] != nil {
		node.cpPool[id].Close()
		delete(node.cpPool, id)
	}
	node.stm.Remove(id)
}
func (node *Node) AddPair(id string, pair *ConnectionPair) {
	if node.cpPool[id] != nil {
		logrus.Warn("recover connection pair ", id)
		node.RemovePair(id)
	}
	node.cpPool[id] = pair
	pair.SetPoolId(id)
	stat := types.Status{
		PairId:    id,
		TargetId:  pair.targetId,
		ImplType:  pair.impl.Code(),
		StartTime: time.Now(),
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
