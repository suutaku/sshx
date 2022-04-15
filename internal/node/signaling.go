package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"path"
	"time"

	"log"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/impl"
	"github.com/suutaku/sshx/pkg/types"
)

func isValidSignalingInfo(input types.SignalingInfo) bool {
	if input.ID == 0 {
		return false
	}
	if input.Source == "" {
		return false
	}
	if input.Target == "" {
		return false
	}
	if input.Source == input.Target {
		return false
	}
	logrus.Debugf(" id %d source %s target %s flag %d", input.ID, input.Source, input.Target, input.Flag)
	return true
}

func poolId(info types.SignalingInfo) string {
	if info.ID == 0 {
		panic("SignalingInfo Id was empty")
	}
	return fmt.Sprintf("conn_%d", info.ID)
}

func (node *Node) push(info types.SignalingInfo) error {
	if !isValidSignalingInfo(info) {
		panic("invalid SignalingInfo")
	}
	node.sigPush <- info
	return nil
}

func (node *Node) ServeSignaling() {

	// pull loop
	go func() {
		client := http.Client{
			Timeout: 10 * time.Second,
		}
		for {
			res, err := client.Get(node.ConfManager.Conf.SignalingServerAddr +
				path.Join("/", "pull", node.ConfManager.Conf.ID))
			if err != nil {
				continue
			}
			var info types.SignalingInfo
			if err = gob.NewDecoder(res.Body).Decode(&info); err != nil {
				if err != nil {
					res.Body.Close()
					continue
				}
			}
			res.Body.Close()
			node.sigPull <- info
			logrus.Debug("pull ok")
		}
	}()

	for {
		client := http.Client{
			Timeout: 10 * time.Second,
		}
		select {
		case info := <-node.sigPush:
			go func() {
				logrus.Debug("start push")
				buf := bytes.NewBuffer(nil)
				if err := gob.NewEncoder(buf).Encode(info); err != nil {
					log.Print(err)
					return
				}
				resp, err := client.Post(node.ConfManager.Conf.SignalingServerAddr+
					path.Join("/", "push", info.Target), "application/binary", buf)
				if err != nil {
					log.Print(err)
					return
				}
				if resp.StatusCode != http.StatusOK {
					log.Println("push to ", info.Target, "faild")
					return
				}
				logrus.Debug("push ok")
			}()
		case info := <-node.sigPull:
			switch info.Flag {
			case types.SIG_TYPE_OFFER:
				cvt := impl.CoreRequest{
					Type: info.RemoteRequestType,
				}
				iface := impl.GetImpl(cvt.GetAppCode())
				param := impl.ImplParam{
					Config: *node.ConfManager.Conf,
					PairId: poolId(info),
				}
				iface.Init(param)
				pair := NewConnectionPair(node.ConfManager.Conf.RTCConf, iface, node.ConfManager.Conf.ID, info.Source)
				node.AddPair(poolId(info), pair)
				err := node.GetPair(poolId(info)).Response(info)
				if err != nil {
					log.Fatal(err)
					continue
				}
				node.GetPair(poolId(info)).PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
					logrus.Debug("send candidate")
					node.SignalCandidate(info, info.Source, c)
				})

				awser := node.GetPair(poolId(info)).Anwser(info)
				if awser != nil {
					node.push(*awser)
				}
			case types.SIG_TYPE_CANDIDATE:
				logrus.Debug("add candidate")
				node.GetPair(poolId(info)).AddCandidate(&webrtc.ICECandidateInit{Candidate: string(info.Candidate)}, info.ID)
			case types.SIG_TYPE_ANSWER:
				err := node.GetPair(poolId(info)).MakeConnection(info)
				if err != nil {
					log.Fatal(err)
				}
			case types.SIG_TYPE_UNKNOWN:
				log.Fatal("unknow signaling type")
			}
		}
	}
}

func (node *Node) SignalCandidate(info types.SignalingInfo, target string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	if node.cpPool[poolId(info)] == nil {
		return
	}
	cadInfo := types.SignalingInfo{
		Flag:              types.SIG_TYPE_CANDIDATE,
		Source:            node.ConfManager.Conf.ID,
		Candidate:         []byte(c.ToJSON().Candidate),
		ID:                node.cpPool[poolId(info)].Id,
		RemoteRequestType: info.RemoteRequestType,
		Target:            target,
	}
	node.push(cadInfo)
}
