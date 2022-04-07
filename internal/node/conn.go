package node

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/conf"
)

type ConnectionPair struct {
	*webrtc.PeerConnection
	LocalSSHConnection *net.Conn
	ID                 int64
	Context            context.Context
	Type               string
	Exit               chan int
}

func NewConnectionPair(cnf webrtc.Configuration, sc *net.Conn, cType string) *ConnectionPair {

	pc, err := webrtc.NewPeerConnection(cnf)
	if err != nil {
		logrus.Error("rtc error:", err)
		return nil
	}
	cp := ConnectionPair{
		PeerConnection:     pc,
		LocalSSHConnection: sc,
		ID:                 time.Now().UnixNano(),
		Type:               cType,
		Exit:               make(chan int),
	}

	cp.PeerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		logrus.Debug("peer connection ice state change: ", state.String())
		if state.String() == webrtc.ICEConnectionStateDisconnected.String() ||
			state.String() == webrtc.ICEConnectionStateFailed.String() ||
			state.String() == webrtc.ICEConnectionStateClosed.String() {
			cp.Close()
		}
	})

	if cType == "_server" {
		cp.PeerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			logrus.Debug("peer connection connection state change 1 : ", state.String())
			if state.String() == webrtc.PeerConnectionStateFailed.String() ||
				state.String() == webrtc.PeerConnectionStateDisconnected.String() ||
				state.String() == webrtc.PeerConnectionStateClosed.String() {
				cp.Close()
			}
		})
		dc, err := cp.PeerConnection.CreateDataChannel("data", nil)
		if err != nil {
			logrus.Error("create dc failed:", err)
			cp.Close()
		}
		dc.OnOpen(func() {
			cp.Exit <- 0 // exit client pull loop
			logrus.Debug("wrap connection")
			_, err := io.Copy(&sendWrap{dc}, *(cp.LocalSSHConnection))
			if err != nil {
				logrus.Error(err)
			}
			cp.Close()
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			_, err := (*cp.LocalSSHConnection).Write(msg.Data)
			if err != nil {
				logrus.Error("sock write failed:", err)
				cp.Close()
				return
			}
		})
		dc.OnClose(func() {
			cp.Close()
			logrus.Debug("Data channel closed")
		})
	} else {
		cp.PeerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			logrus.Debug("peer connection state change 2 : ", state.String())
			if state.String() == webrtc.PeerConnectionStateFailed.String() ||
				state.String() == webrtc.PeerConnectionStateDisconnected.String() ||
				state.String() == webrtc.PeerConnectionStateClosed.String() {
				cp.Close()
			}
		})
		cp.PeerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
			//dc.Lock()
			dc.OnOpen(func() {
				logrus.Debug("wrap connection")
				io.Copy(&sendWrap{dc}, *(cp.LocalSSHConnection))
				cp.Close()
			})
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				_, err := (*cp.LocalSSHConnection).Write(msg.Data)
				if err != nil {
					logrus.Error("sock write failed:", err)
					cp.Close()
					return
				}
			})
			dc.OnClose(func() {
				cp.Close()
			})
			//dc.Unlock()
		})
	}

	return &cp
}

func (cp *ConnectionPair) Anwser(v conf.ConnectInfo, id string) *conf.ConnectInfo {
	if err := cp.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  v.SDP,
	}); err != nil {
		logrus.Error("anwser rtc error:", err)
		cp.Close()
		return nil
	}
	answer, err := cp.PeerConnection.CreateAnswer(nil)
	if err != nil {
		logrus.Error("anwser rtc error:", err)
		cp.Close()
		return nil
	}

	err = cp.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		cp.Close()
		return nil
	}
	r := conf.ConnectInfo{
		Flag:      conf.FLAG_ANWER,
		SDP:       answer.SDP,
		Source:    id,
		Timestamp: v.Timestamp,
	}
	return &r
}

func (cp *ConnectionPair) Offer(id string) *conf.ConnectInfo {

	offer, err := cp.PeerConnection.CreateOffer(nil)
	if err != nil {
		logrus.Error("offer create offer error:", err)
		cp.Close()
		return nil
	}
	if err = cp.PeerConnection.SetLocalDescription(offer); err != nil {
		logrus.Error("offer rtc error:", err)
		cp.Close()
		return nil
	}
	info := conf.ConnectInfo{
		Flag:   conf.FLAG_OFFER,
		Source: id,
		SDP:    offer.SDP,
	}
	return &info
}

func (cp *ConnectionPair) MakeConnection(info conf.ConnectInfo) {
	if cp.PeerConnection == nil {
		return
	}
	if err := cp.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  info.SDP,
	}); err != nil {
		logrus.Error("make connection rtc error:", err)
		cp.Close()
		return
	}
}

func (cp *ConnectionPair) Close() {
	if cp.PeerConnection != nil {
		cp.PeerConnection.Close()
	}
	if cp.LocalSSHConnection != nil {
		(*cp.LocalSSHConnection).Close()
	}
}

func (cp *ConnectionPair) AddCandidate(ca *webrtc.ICECandidateInit, id int64) {
	if cp != nil && id == cp.ID {
		err := cp.PeerConnection.AddICECandidate(*ca)
		if err != nil {
			logrus.Error(err, cp.ID, id)
		}
	} else {
		logrus.Error("Dismatched candidate id ", cp.ID, id)
	}
}

func (cp *ConnectionPair) IsRemoteDscripterSet() bool {
	return !(cp.PeerConnection.RemoteDescription() == nil)
}
