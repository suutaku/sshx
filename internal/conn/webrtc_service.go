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
	BaseConnectionService
	sigPull             chan types.SignalingInfo
	sigPush             chan types.SignalingInfo
	conf                webrtc.Configuration
	signalingServerAddr string
}

func NewWebRTCService(id, signalingServerAddr string, conf webrtc.Configuration) *WebRTCService {
	return &WebRTCService{
		sigPull:               make(chan types.SignalingInfo, 128),
		sigPush:               make(chan types.SignalingInfo, 128),
		conf:                  conf,
		signalingServerAddr:   signalingServerAddr,
		BaseConnectionService: *NewBaseConnectionService(id),
	}
}

func (wss *WebRTCService) Start() error {
	logrus.Debug("start webrtc service")
	wss.BaseConnectionService.Start()
	go wss.ServeSignaling()

	return nil
}

func (wss *WebRTCService) CreateConnection(sender *impl.Sender, sock net.Conn, poolId types.PoolId) error {
	err := wss.BaseConnectionService.CreateConnection(sender, sock, poolId)
	if err != nil {
		return err
	}
	iface := sender.GetImpl()
	if iface == nil {
		return fmt.Errorf("unknown impl")
	}
	if !sender.Detach {
		iface.SetConn(sock)
	}

	pair := NewWebRTC(wss.conf, iface, wss.id, iface.HostId(), poolId, CONNECTION_DRECT_OUT, &wss.CleanChan)
	if pair == nil {
		return fmt.Errorf("cannot create pair")
	}

	err = pair.Dial()
	if err != nil {
		return err
	}
	if iface.IsNeedConnect() {
		logrus.Debug("create connection for ", impl.GetImplName(iface.Code()))
		info, err := pair.Offer(string(iface.HostId()), sender.Type)
		if err != nil {
			return err
		}
		pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
			// set condiate pool it direction to in for server
			if wss.GetPair(info.Id.String(pair.Direction())) == nil {
				return
			}
			info.Id.Direction = pair.Direction()
			wss.SignalCandidate(info, info.Target, c)
		})
		err = wss.push(info)
		if err != nil {
			sock.Close()
			return err
		}
	} else {
		logrus.Error("NOT create connection for ", impl.GetImplName(iface.Code()))
	}

	logrus.Debug("ready to put piar ", pair.poolId.String(pair.Direction()))
	err = wss.AddPair(pair)
	if err != nil {
		return err
	}
	if !sender.Detach {
		logrus.Warn("waitting pair send exit message")
		<-pair.Exit
		logrus.Warn("pair send exit message")
	}
	return nil
}

func (wss *WebRTCService) DestroyConnection(tmp *impl.Sender) error {
	pair := wss.GetPair(string(tmp.PairId))
	if pair == nil {
		return fmt.Errorf("cannot get pair for %s", string(tmp.PairId))
	}
	if pair.GetImpl().Code() == tmp.GetAppCode() {
		wss.RemovePair(CleanRequest{string(tmp.PairId), (&WebRTC{}).Name()})
	}
	return nil
}

func (wss *WebRTCService) isValidSignalingInfo(input types.SignalingInfo) bool {
	if input.Id.Raw() == 0 {
		return false
	}
	if input.Source == "" {
		return false
	}
	if input.Target == "" {
		return false
	}
	return true
}

func (wss *WebRTCService) push(info types.SignalingInfo) error {
	if !wss.isValidSignalingInfo(info) {
		return fmt.Errorf("invalid SignalingInfo")
	}
	wss.sigPush <- info
	return nil
}

func (wss *WebRTCService) ServeOfferInfo(info types.SignalingInfo) {
	if !wss.isValidSignalingInfo(info) {
		logrus.Error("invalid SignalingInfo")
		return
	}
	cvt := impl.Sender{
		Type: info.RemoteRequestType,
	}
	iface := impl.GetImpl(cvt.GetAppCode())
	if iface == nil {
		logrus.Error("unknow impl for IMCODE: ", cvt.GetAppCode())
		return
	}
	iface.SetHostId(info.Source)
	// set candidate pool id direction to out for self(server)
	pair := NewWebRTC(wss.conf, iface, wss.id, info.Source, info.Id, CONNECTION_DRECT_IN, &wss.CleanChan)
	// set candidate pool id direction to out for client
	err := pair.Response()
	if err != nil {
		logrus.Error(err)
		return
	}
	awser, err := pair.Anwser(info)
	if err != nil {
		logrus.Error("pair create a nil anwser")
		return
	}

	pair.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		logrus.Debug("send candidate")
		// set candidate pool id direction to out for client
		info.Id.Direction = pair.Direction()
		wss.SignalCandidate(info, info.Source, c)
	})
	wss.push(awser)
	err = wss.AddPair(pair)
	if err != nil {
		logrus.Error(err)
		return
	}
}

func (wss *WebRTCService) ServePush(info types.SignalingInfo) {
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
	logrus.Debug(wss.signalingServerAddr +
		path.Join("/", "push", info.Target))
}

func (wss *WebRTCService) ServeCandidateInfo(info types.SignalingInfo) {
	info.Id.Direction = ^info.Id.Direction & 0x01
	pair := wss.GetPair(info.Id.String(info.Id.Direction))
	if pair == nil {
		logrus.Warn("pair ", info.Id.String(info.Id.Direction), " was empty, cannot serve candidate")
		return
	}
	pair.(*WebRTC).AddCandidate(&webrtc.ICECandidateInit{Candidate: string(info.Candidate)}, info.Id)
}

func (wss *WebRTCService) ServeAnwserInfo(info types.SignalingInfo) {
	// set candidate pool id direction to out for self(client)
	pair := wss.GetPair(info.Id.String(CONNECTION_DRECT_OUT))
	if pair == nil {
		logrus.Error("pair for id ", info.Id.String(CONNECTION_DRECT_OUT), " was empty, cannot serve anwser")
		return
	}
	err := pair.(*WebRTC).MakeConnection(info)
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
			wss.sigPull <- info
		}
	}()

	for wss.running {
		select {
		case info := <-wss.sigPush:
			go wss.ServePush(info)
		case info := <-wss.sigPull:
			switch info.Flag {
			case types.SIG_TYPE_OFFER:
				// server side
				go wss.ServeOfferInfo(info)
			case types.SIG_TYPE_CANDIDATE:
				// common side
				go wss.ServeCandidateInfo(info)
			case types.SIG_TYPE_ANSWER:
				// client side
				go wss.ServeAnwserInfo(info)
			case types.SIG_TYPE_UNKNOWN:
				logrus.Error("unknow signaling type")
			}
		}
	}
}

func (wss *WebRTCService) SignalCandidate(info types.SignalingInfo, target string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	cadInfo := types.SignalingInfo{
		Flag:              types.SIG_TYPE_CANDIDATE,
		Source:            wss.id,
		Candidate:         []byte(c.ToJSON().Candidate),
		Id:                info.Id,
		RemoteRequestType: info.RemoteRequestType,
		Target:            target,
	}
	wss.push(cadInfo)
}
