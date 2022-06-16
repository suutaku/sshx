package node

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func (node *Node) ServeTCP() {
	listenner, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", node.confManager.Conf.LocalTCPPort))
	if err != nil {
		logrus.Error(err)
		panic(err)
	}
	defer listenner.Close()
	for node.running {
		sock, err := listenner.Accept()
		if err != nil {
			logrus.Error(err)
			continue
		}
		tmp := impl.Sender{}
		err = gob.NewDecoder(sock).Decode(&tmp)
		if err != nil {
			logrus.Debug("read not ok", err)
			sock.Close()
			continue
		}
		switch tmp.GetOptionCode() {
		case types.OPTION_TYPE_UP:

			logrus.Debug("up option")
			err := node.connMgr.CreateConnection(tmp, sock)
			if err != nil {
				sock.Close()
				logrus.Error(err)
			}

		case types.OPTION_TYPE_DOWN:
			logrus.Debug("down option")
			err := node.connMgr.DestroyConnection(tmp)
			if err != nil {
				logrus.Error(err)
			}

		case types.OPTION_TYPE_STAT:
			logrus.Debug("stat option")
			res := node.connMgr.Status()
			err = gob.NewEncoder(sock).Encode(res)
			if err != nil {
				logrus.Error(err)
			}
		case types.OPTION_TYPE_ATTACH:
			logrus.Debug("attach option")
			err := node.connMgr.AttachConnection(tmp, sock)
			if err != nil {
				logrus.Error(err)
			}
		}
	}
}
