package node

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func (node *Node) ServeTCP() {
	listenner, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", node.ConfManager.Conf.LocalTCPPort))
	if err != nil {
		logrus.Error(err)
		panic(err)
	}
	for {
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
			iface := tmp.GetImpl()
			if iface == nil {
				logrus.Error("unknown impl")
				sock.Close()
				break
			}
			if !tmp.Detach {
				iface.SetConn(sock)
			}
			iface.Init()
			logrus.Debug("up option")
			pair := NewConnectionPair(node.ConfManager.Conf.RTCConf, iface, node.ConfManager.Conf.ID, iface.HostId())
			pair.Dial()
			info := pair.Offer(string(iface.HostId()), tmp.Type)
			node.AddPair(poolId(info), pair)
			err = node.push(info)
			if err != nil {
				sock.Close()
				break
			}
			pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
				node.SignalCandidate(info, iface.HostId(), c)
			})
			if !tmp.Detach {
				go pair.ResponseTCP(tmp)
			}
		case types.OPTION_TYPE_DOWN:
			logrus.Debug("down option")
			node.RemovePair(string(tmp.PairId))
		case types.OPTION_TYPE_STAT:
			logrus.Debug("stat option")
			res := node.stm.Get()
			err = gob.NewEncoder(sock).Encode(res)
			if err != nil {
				logrus.Error(err)
			}
		}

	}
}
