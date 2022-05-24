package node

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
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

func poolId(info *types.SignalingInfo) string {
	if info.ID == 0 {
		panic("info id was empty")
	}
	return fmt.Sprintf("conn_%d", info.ID)
}

func (node *Node) push(info *types.SignalingInfo) error {
	if info == nil {
		return fmt.Errorf("empty target id")
	}
	if !isValidSignalingInfo(*info) {
		logrus.Error("invalid SignalingInfo")
	}
	node.sigPush <- info
	return nil
}

func (node *Node) ServeOfferInfo(info *types.SignalingInfo) {
	cvt := impl.Sender{
		Type: info.RemoteRequestType,
	}
	iface := impl.GetImpl(cvt.GetAppCode())
	iface.Init()
	pair := NewConnectionPair(node.ConfManager.Conf.RTCConf, iface, node.ConfManager.Conf.ID, info.Source, &node.CleanChan)
	pair.ResetPoolId(info.ID)
	node.AddPair(pair)
	err := pair.Response(info)
	if err != nil {
		logrus.Error(err)
		return
	}
	pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		logrus.Debug("send candidate")
		node.SignalCandidate(info, info.Source, c)
	})

	awser := pair.Anwser(info)
	if awser == nil {
		logrus.Error("pair create a nil anwser")
		return
	}
	node.push(awser)
}

func (node *Node) ServePush(info *types.SignalingInfo) {
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(info); err != nil {
		logrus.Error(err)
		return
	}
	resp, err := http.Post(node.ConfManager.Conf.SignalingServerAddr+
		path.Join("/", "push", info.Target), "application/binary", buf)
	if err != nil {
		logrus.Error(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		logrus.Errorln("push to ", info.Target, "faild")
		return
	}
}

func (node *Node) ServeCandidateInfo(info *types.SignalingInfo) {
	logrus.Debug("add candidate")
	pair := node.GetPair(poolId(info))
	if pair == nil {
		logrus.Warn("pair ", poolId(info), "was empty, cannot serve candidate")
		return
	}
	pair.AddCandidate(&webrtc.ICECandidateInit{Candidate: string(info.Candidate)}, info.ID)
}

func (node *Node) ServeAnwserInfo(info *types.SignalingInfo) {
	pair := node.GetPair(poolId(info))
	if pair == nil {
		logrus.Warn("pair for id ", poolId(info), " was empty, cannot serve anwser")
		return
	}
	err := pair.MakeConnection(info)
	if err != nil {
		logrus.Error(err)
	}
}

func (node *Node) ServeSignaling() {

	// pull loop
	go func() {
		for node.running {
			res, err := http.Get(node.ConfManager.Conf.SignalingServerAddr +
				path.Join("/", "pull", node.ConfManager.Conf.ID))
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			var info types.SignalingInfo
			if err = gob.NewDecoder(res.Body).Decode(&info); err != nil {
				if err != nil {
					res.Body.Close()
					time.Sleep(1 * time.Second)
					continue
				}
			}
			res.Body.Close()
			node.sigPull <- &info
		}
	}()

	for node.running {
		select {
		case info := <-node.sigPush:
			go node.ServePush(info)
		case info := <-node.sigPull:
			switch info.Flag {
			case types.SIG_TYPE_OFFER:
				go node.ServeOfferInfo(info)
			case types.SIG_TYPE_CANDIDATE:
				go node.ServeCandidateInfo(info)
			case types.SIG_TYPE_ANSWER:
				go node.ServeAnwserInfo(info)
			case types.SIG_TYPE_UNKNOWN:
				logrus.Error("unknow signaling type")
			}
		}
	}
}

func (node *Node) SignalCandidate(info *types.SignalingInfo, target string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	if node.cpPool[poolId(info)] == nil {
		return
	}
	cadInfo := &types.SignalingInfo{
		Flag:              types.SIG_TYPE_CANDIDATE,
		Source:            node.ConfManager.Conf.ID,
		Candidate:         []byte(c.ToJSON().Candidate),
		ID:                info.ID,
		RemoteRequestType: info.RemoteRequestType,
		Target:            target,
	}
	node.push(cadInfo)
}
