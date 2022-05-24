package node

import (
	"encoding/gob"
	"fmt"
	"io"
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

type ConnectionPair struct {
	*webrtc.PeerConnection
	conf     webrtc.Configuration
	Exit     chan error
	impl     impl.Impl
	nodeId   string
	targetId string
	stmChan  *chan string
	poolId   int64
}

func NewConnectionPair(conf webrtc.Configuration, impl impl.Impl, nodeId string, targetId string, stmChan *chan string) *ConnectionPair {
	pc, err := webrtc.NewPeerConnection(conf)
	if err != nil {
		logrus.Error("rtc error:", err)
		return nil
	}
	return &ConnectionPair{
		PeerConnection: pc,
		conf:           conf,
		Exit:           make(chan error, 10),
		impl:           impl,
		nodeId:         nodeId,
		targetId:       targetId,
		stmChan:        stmChan,
		poolId:         time.Now().UnixNano(),
	}
}

func (pair *ConnectionPair) PoolId() int64 {
	return pair.poolId
}

func (pair *ConnectionPair) PoolIdStr() string {
	return fmt.Sprintf("conn_%d", pair.poolId)
}

func (pair *ConnectionPair) GetImpl() impl.Impl {
	return pair.impl
}

func (pair *ConnectionPair) ResetPoolId(id int64) {
	logrus.Debug("reset pool id from ", pair.poolId, " to ", id)
	pair.poolId = id
}

// create responser
func (pair *ConnectionPair) Response(info *types.SignalingInfo) error {
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
			err := pair.impl.Response()
			if err != nil {
				logrus.Error(err)
				pair.Exit <- err
				pair.Close()
				return
			}
			pair.Exit <- err
			logrus.Info("data channel open 2")
			io.Copy(&Wrapper{dc}, pair.impl.Reader())
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
			logrus.Debug("data channel close")
			pair.Exit <- nil
			pair.Close()
		})
	})

	pair.PeerConnection = peer
	return nil
}

// create dialer
func (pair *ConnectionPair) Dial() error {
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
	go pair.impl.Dial()
	dc.OnOpen(func() {
		logrus.Info("data channel open 1")
		pair.Exit <- nil
		_, err := io.Copy(&Wrapper{dc}, pair.impl.Reader())
		if err != nil {
			logrus.Error(err)
			pair.Exit <- err
			dc.Close()
			pair.Close()
		}
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
		logrus.Info("data channel close")
		pair.Exit <- fmt.Errorf("data channel close")
		pair.Close()
		logrus.Debug("data channel closed")
	})
	pair.PeerConnection = peer
	return nil
}
func (pair *ConnectionPair) Close() {
	logrus.Debug("close pair")
	if pair.PeerConnection != nil {
		pair.PeerConnection.Close()
		(*pair.stmChan) <- pair.PoolIdStr()
	}
	if pair.impl != nil {
		pair.impl.Close()
	}
}

func (pair *ConnectionPair) Offer(target string, reType int32) *types.SignalingInfo {
	logrus.Debug("pair offer")
	if target == "" {
		return nil
	}
	offer, err := pair.PeerConnection.CreateOffer(nil)
	if err != nil {
		logrus.Error("offer create offer error:", err)
		pair.Close()
		return nil
	}
	if err = pair.PeerConnection.SetLocalDescription(offer); err != nil {
		logrus.Error("offer rtc error:", err)
		pair.Close()
		return nil
	}
	info := types.SignalingInfo{
		ID:                pair.PoolId(),
		Flag:              types.SIG_TYPE_OFFER,
		Target:            pair.targetId,
		SDP:               offer.SDP,
		RemoteRequestType: reType,
		Source:            pair.nodeId,
	}
	return &info
}

func (pair *ConnectionPair) Anwser(info *types.SignalingInfo) *types.SignalingInfo {
	logrus.Debug("pair anwser")
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("anwser rtc error:", err)
		pair.Close()
		return nil
	}
	answer, err := pair.PeerConnection.CreateAnswer(nil)
	if err != nil {
		logrus.Error("anwser rtc error:", err)
		pair.Close()
		return nil
	}

	err = pair.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		logrus.Error(err)
		pair.Close()
		return nil
	}
	return &types.SignalingInfo{
		ID:     info.ID,
		Flag:   types.SIG_TYPE_ANSWER,
		SDP:    answer.SDP,
		Target: pair.targetId,
		Source: pair.nodeId,
	}
}

func (pair *ConnectionPair) MakeConnection(info *types.SignalingInfo) error {
	logrus.Debug("pair make connection")
	if pair == nil || pair.PeerConnection == nil {
		return fmt.Errorf("invalid peer connection")
	}
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("make connection rtc error: ", pair.PoolIdStr(), " ", err)
		pair.Close()
		return err
	}
	pair.Exit <- nil
	return nil
}

func (pair *ConnectionPair) AddCandidate(ca *webrtc.ICECandidateInit, id int64) error {
	if pair != nil && id == pair.PoolId() {
		if !pair.IsRemoteDescriptionSet() {
			logrus.Warn("waiting remote description be set ", pair.PoolIdStr())
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

func (pair *ConnectionPair) IsRemoteDescriptionSet() bool {
	return !(pair.PeerConnection.RemoteDescription() == nil)
}

func (pair *ConnectionPair) ResponseTCP(resp impl.Sender) {
	logrus.Debug("waiting pair signal")
	err := <-pair.Exit
	logrus.Debug("Response TCP")
	if err != nil {
		logrus.Error(err)
		resp.Status = -1
	}
	err = gob.NewEncoder(pair.impl.Writer()).Encode(resp)
	if err != nil {
		logrus.Error(err)
		return
	}
}
