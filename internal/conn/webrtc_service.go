package conn

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

type WebRTCService struct {
	sigPull             chan *types.SignalingInfo
	sigPush             chan *types.SignalingInfo
	CleanChan           chan string
	conf                webrtc.Configuration
	id                  string
	signalingServerAddr string
	cpPool              map[string]*WebRTC
	running             bool
	stm                 *StatManager
	isReady             bool
}

func NewWebRTCService(id, signalingServerAddr string, conf webrtc.Configuration) *WebRTCService {
	return &WebRTCService{
		sigPull:             make(chan *types.SignalingInfo, 128),
		sigPush:             make(chan *types.SignalingInfo, 128),
		CleanChan:           make(chan string, 10),
		conf:                conf,
		id:                  id,
		signalingServerAddr: signalingServerAddr,
		cpPool:              make(map[string]*WebRTC),
	}
}

func (wss *WebRTCService) SetStateManager(stm *StatManager) error {
	wss.stm = stm
	return nil
}

func (wss *WebRTCService) Start() error {
	logrus.Debug("start webrtc service")
	wss.running = true
	wss.isReady = true
	go wss.ServeSignaling()
	go wss.WatchPairs()
	return nil
}

func (wss *WebRTCService) Stop() {
	wss.running = false
	wss.isReady = false
}

func (wss *WebRTCService) IsReady() bool {
	return wss.isReady
}

func (wss *WebRTCService) CreateConnection(sender impl.Sender, sock net.Conn) error {
	iface := sender.GetImpl()
	if iface == nil {
		return fmt.Errorf("unknown impl")

	}
	if !sender.Detach {
		iface.SetConn(sock)
	}
	pair := NewConnectionPair(wss.conf, iface, wss.id, iface.HostId(), &wss.CleanChan)
	pair.Dial()
	info := pair.Offer(string(iface.HostId()), sender.Type)
	wss.AddPair(pair)

	err := wss.push(info)
	if err != nil {
		sock.Close()
		return err
	}
	pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		wss.SignalCandidate(info, iface.HostId(), c)
	})
	if !sender.Detach {
		// fill pair id and send back the 'sender'
		sender.PairId = []byte(wss.poolId(info))
		go pair.ResponseTCP(sender)
	}
	return nil
}

func (wss *WebRTCService) DestroyConnection(tmp impl.Sender) error {
	pair := wss.GetPair(string(tmp.PairId))
	if pair == nil {
		return fmt.Errorf("cannot get pair for %s", string(tmp.PairId))
	}
	if pair.GetImpl().Code() == tmp.GetAppCode() {
		wss.RemovePair(string(tmp.PairId))
	}
	return nil
}

func (wss *WebRTCService) AttachConnection(sender impl.Sender, sock net.Conn) error {
	pair := wss.GetPair(string(sender.PairId))
	if pair == nil {
		return fmt.Errorf("cannot attach impl with id: %s", string(sender.PairId))

	}
	// should assign host id and return
	retSender := impl.NewSender(pair.impl, types.OPTION_TYPE_ATTACH)
	err := gob.NewEncoder(sock).Encode(retSender)
	if err != nil {
		return err
	}
	pair.impl.Attach(sock)
	return nil
}

func (wss *WebRTCService) isValidSignalingInfo(input types.SignalingInfo) bool {
	if input.ID == 0 {
		return false
	}
	if input.Source == "" {
		return false
	}
	if input.Target == "" {
		return false
	}
	// if input.Source == input.Target {
	// 	return false
	// }
	logrus.Debugf(" id %d source %s target %s flag %d", input.ID, input.Source, input.Target, input.Flag)
	return true
}

func (wss *WebRTCService) poolId(info *types.SignalingInfo) string {
	if info.ID == 0 {
		panic("info id was empty")
	}
	return fmt.Sprintf("conn_%d", info.ID)
}

func (wss *WebRTCService) push(info *types.SignalingInfo) error {
	if info == nil {
		logrus.Warn("nothing to push")
		return nil
	}
	if !wss.isValidSignalingInfo(*info) {
		return fmt.Errorf("invalid SignalingInfo")
	}
	wss.sigPush <- info
	return nil
}

func (wss *WebRTCService) ServeOfferInfo(info *types.SignalingInfo) {
	cvt := impl.Sender{
		Type: info.RemoteRequestType,
	}
	iface := impl.GetImpl(cvt.GetAppCode())
	if iface == nil {
		logrus.Warn("unknow impl for IMCODE: ", cvt.GetAppCode())
		return
	}
	iface.SetHostId(info.Source)
	pair := NewConnectionPair(wss.conf, iface, wss.id, info.Source, &wss.CleanChan)
	pair.ResetPoolId(info.ID)
	wss.AddPair(pair)
	err := pair.Response(info)
	if err != nil {
		logrus.Error(err)
		return
	}
	pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		logrus.Debug("send candidate")
		wss.SignalCandidate(info, info.Source, c)
	})

	awser := pair.Anwser(info)
	if awser == nil {
		logrus.Error("pair create a nil anwser")
		return
	}
	wss.push(awser)
}

func (wss *WebRTCService) ServePush(info *types.SignalingInfo) {
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(info); err != nil {
		logrus.Error(err)
		return
	}
	resp, err := http.Post(wss.signalingServerAddr+
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

func (wss *WebRTCService) ServeCandidateInfo(info *types.SignalingInfo) {
	logrus.Debug("add candidate")
	pair := wss.GetPair(wss.poolId(info))
	if pair == nil {
		logrus.Warn("pair ", wss.poolId(info), " was empty, cannot serve candidate")
		return
	}
	pair.AddCandidate(&webrtc.ICECandidateInit{Candidate: string(info.Candidate)}, info.ID)
}

func (wss *WebRTCService) ServeAnwserInfo(info *types.SignalingInfo) {
	pair := wss.GetPair(wss.poolId(info))
	if pair == nil {
		logrus.Error("pair for id ", wss.poolId(info), " was empty, cannot serve anwser")
		return
	}
	err := pair.MakeConnection(info)
	if err != nil {
		logrus.Error(err)
	}
}

func (wss *WebRTCService) ServeSignaling() {

	// pull loop
	go func() {
		for wss.running {
			res, err := http.Get(wss.signalingServerAddr +
				path.Join("/", "pull", wss.id))
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
			wss.sigPull <- &info
		}
	}()

	for wss.running {
		select {
		case info := <-wss.sigPush:
			go wss.ServePush(info)
		case info := <-wss.sigPull:
			switch info.Flag {
			case types.SIG_TYPE_OFFER:
				go wss.ServeOfferInfo(info)
			case types.SIG_TYPE_CANDIDATE:
				go wss.ServeCandidateInfo(info)
			case types.SIG_TYPE_ANSWER:
				go wss.ServeAnwserInfo(info)
			case types.SIG_TYPE_UNKNOWN:
				logrus.Error("unknow signaling type")
			}
		}
	}
}

func (wss *WebRTCService) SignalCandidate(info *types.SignalingInfo, target string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	if wss.cpPool[wss.poolId(info)] == nil {
		return
	}
	cadInfo := &types.SignalingInfo{
		Flag:              types.SIG_TYPE_CANDIDATE,
		Source:            wss.id,
		Candidate:         []byte(c.ToJSON().Candidate),
		ID:                info.ID,
		RemoteRequestType: info.RemoteRequestType,
		Target:            target,
	}
	wss.push(cadInfo)
}

func (wss *WebRTCService) RemovePair(id string) {
	children := wss.stm.GetChildren(id)
	logrus.Debug("ready to clear children ", children)
	// close children
	for _, v := range children {
		if wss.cpPool[v] != nil {
			wss.cpPool[v].Close()
			delete(wss.cpPool, v)
		}
		wss.stm.Remove(id)
	}
	// close parent
	if wss.cpPool[id] != nil {
		wss.cpPool[id].Close()
		delete(wss.cpPool, id)
	}
	wss.stm.Remove(id)
	wss.stm.RemoveParent(id)
}
func (wss *WebRTCService) AddPair(pair *WebRTC) {
	// if node.cpPool[id] != nil {
	// 	logrus.Warn("recover connection pair ", id)
	// 	node.RemovePair(id)
	// }
	if pair == nil {
		return
	}
	wss.cpPool[pair.PoolIdStr()] = pair
	stat := types.Status{
		PairId:    pair.PoolIdStr(),
		TargetId:  pair.targetId,
		ImplType:  pair.impl.Code(),
		StartTime: time.Now(),
	}

	if pair.impl.ParentId() != "" {
		logrus.Debug("add child ", pair.PoolIdStr(), " to ", pair.impl.ParentId())
		stat.ParentPairId = pair.impl.ParentId()
		wss.stm.AddChild(pair.impl.ParentId(), pair.PoolIdStr())
	}
	wss.stm.Put(stat)

}

func (wss *WebRTCService) GetPair(id string) *WebRTC {
	return wss.cpPool[id]
}

func (wss *WebRTCService) WatchPairs() {
	for wss.running {
		pairId := <-wss.CleanChan
		wss.RemovePair(pairId)
		logrus.Debug("clean request from clean channel ", pairId)
	}
}
