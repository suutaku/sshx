package node

import (
	"github.com/suutaku/sshx/internal/conn"
	"github.com/suutaku/sshx/pkg/conf"
)

type Node struct {
	confManager *conf.ConfManager
	running     bool
	connMgr     *conn.ConnectionManager
}

func NewNode(home string) *Node {
	cm := conf.NewConfManager(home)
	enabledService := make([]conn.ConnectionService, 0)
	enabledService = append(enabledService, conn.NewWebRTCService(cm.Conf.ID, cm.Conf.SignalingServerAddr, cm.Conf.RTCConf))
	return &Node{
		confManager: cm,
		connMgr:     conn.NewConnectionManager(enabledService),
	}
}

func (node *Node) Start() {
	node.running = true
	go node.connMgr.Start()
	node.ServeTCP()
}

func (node *Node) Stop() {
	node.running = false
	node.connMgr.Stop()
}
