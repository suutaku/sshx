package node

import (
	"encoding/gob"
	"fmt"
	"io"
	"time"

	"github.com/suutaku/sshx/internal/impl"
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
	Id          int64
	conf        webrtc.Configuration
	Exit        chan error
	impl        impl.Impl
	nodeId      string
	targetId    string
	isRemoteSet bool
}

func NewConnectionPair(conf webrtc.Configuration, impl impl.Impl, nodeId string, targetId string) *ConnectionPair {
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
	}
}

func (pair *ConnectionPair) SetId(id int64) {
	pair.Id = id
	debug := fmt.Sprintf("conn_%d", id)
	pair.impl.SetPairId(debug)
}

// create responser
func (pair *ConnectionPair) Response(info types.SignalingInfo) error {
	pair.Id = info.ID
	logrus.Debug("pair response")
	peer, err := webrtc.NewPeerConnection(pair.conf)
	if err != nil {
		pair.Exit <- err
		logrus.Print(err)
		return err
	}
	peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		//dc.Lock()
		dc.OnOpen(func() {
			logrus.Info("data channel open 2")
			err := pair.impl.Response()
			if err != nil {
				logrus.Error(err)
				pair.Exit <- err
				return
			}
			pair.Exit <- err
			io.Copy(&Wrapper{dc}, *pair.impl.Conn())
			dc.Close()
			pair.Close()
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			logrus.Debug("message comes")
			if pair.impl == nil || pair.impl.Conn() == nil {
				logrus.Debug("on message impl or impl conn was empty")
				pair.Close()
				return
			}
			_, err := (*pair.impl.Conn()).Write(msg.Data)
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
	dc.OnOpen(func() {
		logrus.Info("data channel open 1")
		pair.Exit <- nil
		_, err := io.Copy(&Wrapper{dc}, *pair.impl.Conn())
		if err != nil {
			logrus.Error(err)
			pair.Exit <- err
			dc.Close()
			pair.Close()
		}
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		_, err := (*pair.impl.Conn()).Write(msg.Data)
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
	if pair.PeerConnection != nil {
		pair.PeerConnection.Close()
	}
	if pair.impl != nil {
		pair.impl.Close()
	}
}

func (pair *ConnectionPair) Offer(target string, reType int32) *types.SignalingInfo {
	logrus.Debug("pair offer")
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
		ID:                time.Now().UnixNano(),
		Flag:              types.SIG_TYPE_OFFER,
		Target:            pair.targetId,
		SDP:               offer.SDP,
		RemoteRequestType: reType,
		Source:            pair.nodeId,
	}
	pair.SetId(info.ID)
	return &info
}

func (pair *ConnectionPair) Anwser(info types.SignalingInfo) *types.SignalingInfo {
	pair.SetId(info.ID)
	logrus.Debug("pair anwser")
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("anwser rtc error:", err)
		pair.Close()
		return nil
	}
	pair.isRemoteSet = true
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

func (pair *ConnectionPair) MakeConnection(info types.SignalingInfo) error {
	logrus.Debug("pair make connection")
	if pair == nil || pair.PeerConnection == nil {
		return fmt.Errorf("invalid peer connection")
	}
	if err := pair.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("make connection rtc error:", err)
		pair.Close()
		return err
	}
	pair.isRemoteSet = true
	pair.Exit <- nil
	return nil
}

func (pair *ConnectionPair) AddCandidate(ca *webrtc.ICECandidateInit, id int64) error {
	if pair != nil && id == pair.Id {
		for !pair.isRemoteSet {
			logrus.Warn("waiting remote description be set")
			time.Sleep(200 * time.Millisecond)
		}
		err := pair.PeerConnection.AddICECandidate(*ca)
		if err != nil {
			logrus.Error(err, pair.Id, id)
			return err
		}
	} else {
		return fmt.Errorf("dismatched candidate id")
	}
	return nil
}

func (pair *ConnectionPair) IsRemoteDscripterSet() bool {
	return !(pair.PeerConnection.RemoteDescription() == nil)
}

func (pair *ConnectionPair) ResponseTCP(resp impl.CoreResponse) {
	logrus.Debug("waiting pair signal")
	err := <-pair.Exit
	logrus.Debug("Response TCP")
	if err != nil {
		logrus.Error(err)
		resp.Status = -1
	}

	enc := gob.NewEncoder(*pair.impl.Conn())
	err = enc.Encode(resp)
	if err != nil {
		logrus.Error(err)
		return
	}
}
