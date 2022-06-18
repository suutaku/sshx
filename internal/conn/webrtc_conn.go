package conn

import (
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
)

type Wrapper struct {
	*webrtc.DataChannel
}

func (s *Wrapper) Write(b []byte) (int, error) {
	err := s.DataChannel.Send(b)
	return len(b), err
}

type WebRTC struct {
	BaseConnection
	*webrtc.PeerConnection
	conf    webrtc.Configuration
	stmChan *chan string
}

func NewWebRTC(conf webrtc.Configuration, impl impl.Impl, nodeId string, targetId string, poolId types.PoolId, direct int32, stmChan *chan string) *WebRTC {
	pc, err := webrtc.NewPeerConnection(conf)
	if err != nil {
		logrus.Error("rtc error:", err)
		return nil
	}
	ret := &WebRTC{
		PeerConnection: pc,
		conf:           conf,
		BaseConnection: *NewBaseConnection(impl, nodeId, targetId, poolId, direct, impl.Code()),
		stmChan:        stmChan,
	}
	ret.impl.SetPairId(poolId.String(ret.Direction()))
	return ret
}

func (pair *WebRTC) Name() string {
	if t := reflect.TypeOf(pair); t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	} else {
		return t.Name()
	}
}

// create responser
func (pair *WebRTC) Response() error {
	logrus.Debug("pair response")
	peer, err := webrtc.NewPeerConnection(pair.conf)
	if err != nil {
		pair.Exit <- err
		logrus.Print(err)
		pair.Close()
		return err
	}
	peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		//dc.Lock()
		dc.OnOpen(func() {
			err := pair.BaseConnection.Response()
			if err != nil {
				logrus.Error(err)
				pair.Exit <- err
				pair.Close()
				return
			}
			pair.Exit <- err
			pair.Ready()
			logrus.Info("data channel open 2")
			n, err := io.Copy(&Wrapper{dc}, pair.impl.Reader())
			for dc.BufferedAmount() > 0 {
				time.Sleep(100 * time.Millisecond)
			}
			logrus.Info("trans2 ", n, err)
			pair.Exit <- fmt.Errorf("io copy break")
			dc.Close()
			pair.Close()
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			if pair.impl == nil {
				pair.Close()
				return
			}
			_, err := pair.impl.Writer().Write(msg.Data)
			if err != nil {
				logrus.Error("sock write failed:", err)
				pair.Close()
				return
			}
		})
		dc.OnClose(func() {
			logrus.Debug("data channel close 2")
			pair.Exit <- nil
			pair.Close()
		})
	})

	pair.PeerConnection = peer
	return nil
}

// create dialer
func (pair *WebRTC) Dial() error {
	logrus.Debug("pair dial")
	peer, err := webrtc.NewPeerConnection(pair.conf)
	if err != nil {
		logrus.Error(err)
		return err
	}
	dc, err := peer.CreateDataChannel("data", nil)
	if err != nil {
		pair.Close()
		return err
	}
	go func() {
		err := pair.BaseConnection.Dial()
		if err != nil {
			logrus.Error(err)
			pair.Exit <- err
			dc.Close()
			pair.Close()
		}
	}()
	dc.OnOpen(func() {
		logrus.Info("data channel open 1")
		pair.Exit <- nil
		pair.Ready()
		// hangs
		n, err := io.Copy(&Wrapper{dc}, pair.impl.Reader())
		if err != nil {
			logrus.Error(err)
		}
		for dc.BufferedAmount() > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		logrus.Info("trans1 ", n, err)
		pair.Exit <- err
		dc.Close()
		pair.Close()
		logrus.Info("data channel close 1")
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if pair.impl == nil {
			pair.Close()
			return
		}
		_, err := pair.impl.Writer().Write(msg.Data)
		if err != nil {
			logrus.Error("sock write failed:", err)
			pair.Close()
		}
	})
	dc.OnClose(func() {
		logrus.Info("data channel close 1")
		pair.Exit <- fmt.Errorf("data channel close")
		pair.Close()
		logrus.Debug("data channel closed")
	})
	pair.PeerConnection = peer
	return nil
}
func (pair *WebRTC) Close() {
	logrus.Debug("close pair")
	if pair.PeerConnection != nil {
		pair.PeerConnection.Close()
		(*pair.stmChan) <- pair.poolId.String(pair.Direction())
	}
	if pair.impl != nil {
		pair.impl.Close()
	}
}

func (pair *WebRTC) Offer(target string, reType int32) (types.SignalingInfo, error) {
	var info types.SignalingInfo
	logrus.Debug("pair offer")
	if target == "" {
		return info, fmt.Errorf("target was empty")
	}
	offer, err := pair.PeerConnection.CreateOffer(nil)
	if err != nil {
		pair.Close()
		return info, err
	}
	if err = pair.PeerConnection.SetLocalDescription(offer); err != nil {
		pair.Close()
		return info, err
	}
	ret := types.SignalingInfo{
		Id:                pair.poolId,
		Flag:              types.SIG_TYPE_OFFER,
		Target:            pair.targetId,
		SDP:               offer.SDP,
		RemoteRequestType: reType,
		Source:            pair.nodeId,
	}
	return ret, nil
}

func (pair *WebRTC) Anwser(info types.SignalingInfo) (types.SignalingInfo, error) {
	logrus.Debug("pair anwser")
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  info.SDP,
	}); err != nil {
		pair.Close()
		return info, err
	}
	answer, err := pair.PeerConnection.CreateAnswer(nil)
	if err != nil {
		pair.Close()
		return info, err
	}

	err = pair.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		logrus.Error(err)
		pair.Close()
		return info, err
	}
	ret := types.SignalingInfo{
		Id:     info.Id,
		Flag:   types.SIG_TYPE_ANSWER,
		SDP:    answer.SDP,
		Target: pair.targetId,
		Source: pair.nodeId,
	}
	return ret, nil
}

func (pair *WebRTC) MakeConnection(info types.SignalingInfo) error {
	logrus.Debug("pair make connection")
	if pair == nil || pair.PeerConnection == nil {
		return fmt.Errorf("invalid peer connection")
	}
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("make connection rtc error: ", pair.poolId.String(pair.Direction()), " ", err)
		pair.Close()
		return err
	}
	pair.Exit <- nil
	return nil
}

func (pair *WebRTC) AddCandidate(ca *webrtc.ICECandidateInit, id types.PoolId) error {
	if pair != nil && id.Raw() == pair.PoolId().Raw() {
		if !pair.IsRemoteDescriptionSet() {
			logrus.Warn("waiting remote description be set ", pair.poolId.String(pair.Direction()))
			return fmt.Errorf("remote description NOT set")
		}
		err := pair.PeerConnection.AddICECandidate(*ca)
		if err != nil {
			logrus.Error(err, pair.PoolId(), id)
			return err
		}
	} else {
		return fmt.Errorf("dismatched candidate id")
	}
	return nil
}

func (pair *WebRTC) IsRemoteDescriptionSet() bool {
	return !(pair.PeerConnection.RemoteDescription() == nil)
}
