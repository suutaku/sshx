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
			iface := tmp.GetImpl()
			if iface == nil {
				logrus.Error("unknown impl")
				sock.Close()
				break
			}
			if !tmp.Detach {
				iface.SetConn(sock)
			}
			logrus.Debug("up option")
			pair := NewConnectionPair(node.ConfManager.Conf.RTCConf, iface, node.ConfManager.Conf.ID, iface.HostId(), &node.CleanChan)
			pair.Dial()
			info := pair.Offer(string(iface.HostId()), tmp.Type)
			node.AddPair(pair)

			err = node.push(info)
			if err != nil {
				sock.Close()
				break
			}
			pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
				node.SignalCandidate(info, iface.HostId(), c)
			})
			if !tmp.Detach {
				// fill pair id and send back the 'sender'
				tmp.PairId = []byte(poolId(info))
				go pair.ResponseTCP(tmp)
			}
		case types.OPTION_TYPE_DOWN:
			logrus.Debug("down option")
			pair := node.GetPair(string(tmp.PairId))
			if pair == nil {
				logrus.Warn("cannot get pair for ", string(tmp.PairId))
				return
			}
			if pair.GetImpl().Code() == tmp.GetAppCode() {
				node.RemovePair(string(tmp.PairId))
			}
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
