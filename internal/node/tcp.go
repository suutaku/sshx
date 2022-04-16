package node

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
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
		tmp := impl.CoreRequest{}
		dec := gob.NewDecoder(sock)
		err = dec.Decode(&tmp)
		if err != nil {
			logrus.Debug("read not ok")
			sock.Close()
			continue
		}

		switch tmp.GetOptionCode() {
		case types.OPTION_TYPE_UP:
			iface := impl.GetImpl(tmp.GetAppCode())
			param := impl.ImplParam{
				Conn:   &sock,
				Config: *node.ConfManager.Conf,
			}
			iface.Init(param)
			pair := NewConnectionPair(node.ConfManager.Conf.RTCConf, iface, node.ConfManager.Conf.ID, string(tmp.Payload))
			pair.Dial()
			info := pair.Offer(string(tmp.Payload), tmp.Type)
			node.push(*info)
			pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
				node.SignalCandidate(*info, string(tmp.Payload), c)
			})
			node.AddPair(poolId(*info), pair)
			resp := impl.NewCoreResponse(iface.Code(), types.OPTION_TYPE_UP)
			resp.PairId = []byte(poolId(*info))
			go pair.ResponseTCP(*resp)

		case types.OPTION_TYPE_DOWN:
			node.RemovePair(string(tmp.PairId))
		}

	}
}
