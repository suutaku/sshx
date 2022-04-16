package node

import (
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/types"
)

type Node struct {
	ConfManager *conf.ConfManager
	sigPull     chan types.SignalingInfo
	sigPush     chan types.SignalingInfo
	cpPool      map[string]*ConnectionPair
}

func NewNode(home string) *Node {
	return &Node{
		sigPull:     make(chan types.SignalingInfo, 128),
		sigPush:     make(chan types.SignalingInfo, 128),
		ConfManager: conf.NewConfManager(home),
		cpPool:      make(map[string]*ConnectionPair),
	}
}

func (node *Node) Start() {
	//vncServer := vnc.NewVNC(context.TODO(), node.ConfManager.Conf.VNCConf)
	//go vncServer.Start()
	go node.ServeHTTPAndWS()
	go node.ServeSignaling()
	node.ServeTCP()
}

func (node *Node) RemovePair(id string) {
	if node.cpPool[id] != nil {
		node.cpPool[id].Close()
		delete(node.cpPool, id)
	}
}
func (node *Node) AddPair(id string, pair *ConnectionPair) {
	if node.cpPool[id] != nil {
		logrus.Warn("recover connection pair ", id)
		node.RemovePair(id)
	}
	node.cpPool[id] = pair
}

func (node *Node) GetPair(id string) *ConnectionPair {
	return node.cpPool[id]
}
