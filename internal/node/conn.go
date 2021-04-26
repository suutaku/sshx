package node

import (
	"context"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net"
	"time"
)

type ConnectionPair struct {
	PeerConnection     *webrtc.PeerConnection
	LocalSSHConnection *net.Conn
	ID                 int64
	Context            context.Context
	Type               string
	Exit               chan int
}

func NewConnectionPair(cnf webrtc.Configuration, sc *net.Conn, cType string) *ConnectionPair {

	pc, err := webrtc.NewPeerConnection(cnf)
	if err != nil {
		log.Println("rtc error:", err)
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
		log.Print("pc ice state change: ", state.String())
		if state.String() == webrtc.ICEConnectionStateDisconnected.String() ||
			state.String() == webrtc.ICEConnectionStateFailed.String() ||
			state.String() == webrtc.ICEConnectionStateClosed.String() {
			cp.Close()
		}
	})

	if cType == "_server" {
		cp.PeerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			log.Print("pc connection state change: ", state.String())
			if state.String() == webrtc.PeerConnectionStateFailed.String() ||
				state.String() == webrtc.PeerConnectionStateDisconnected.String() ||
				state.String() == webrtc.PeerConnectionStateClosed.String() {
				cp.Close()
			}
		})
		dc, err := cp.PeerConnection.CreateDataChannel("data", nil)
		if err != nil {
			log.Println("create dc failed:", err)
			cp.Close()
		}
		dc.OnOpen(func() {
			cp.Exit <- 0 // exit client pull loop
			log.Println("wrap to ssh dc")
			io.Copy(&sendWrap{dc}, *(cp.LocalSSHConnection))
			cp.Close()
		})
		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			_, err := (*cp.LocalSSHConnection).Write(msg.Data)
			if err != nil {
				log.Println("sock write failed:", err)
				cp.Close()
				return
			}
		})
		dc.OnClose(func() {
			cp.Close()
			log.Println("Data channel closed")
		})
	} else {
		cp.PeerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			log.Print("pc connection state change: ", state.String())
			if state.String() == webrtc.PeerConnectionStateFailed.String() ||
				state.String() == webrtc.PeerConnectionStateDisconnected.String() ||
				state.String() == webrtc.PeerConnectionStateClosed.String() {
				cp.Close()
			}
		})
		cp.PeerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
			//dc.Lock()
			dc.OnOpen(func() {
				log.Println("wrap to ssh dc")
				io.Copy(&sendWrap{dc}, *(cp.LocalSSHConnection))
				cp.Close()
			})
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				_, err := (*cp.LocalSSHConnection).Write(msg.Data)
				if err != nil {
					log.Println("sock write failed:", err)
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

func (cp *ConnectionPair) Anwser(v ConnectInfo, id string) *ConnectInfo {
	if err := cp.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  v.SDP,
	}); err != nil {
		log.Println("anwser rtc error:", err)
		cp.Close()
		return nil
	}
	answer, err := cp.PeerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("anwser rtc error:", err)
		cp.Close()
		return nil
	}

	err = cp.PeerConnection.SetLocalDescription(answer)
	if err != nil {
		cp.Close()
		return nil
	}
	r := ConnectInfo{
		Flag:   FLAG_ANWER,
		SDP:    answer.SDP,
		Source: id,
	}
	return &r
}

func (cp *ConnectionPair) Offer(id string) *ConnectInfo {

	offer, err := cp.PeerConnection.CreateOffer(nil)
	if err != nil {
		log.Println("offer create offer error:", err)
		cp.Close()
		return nil
	}
	if err = cp.PeerConnection.SetLocalDescription(offer); err != nil {
		log.Println("offer rtc error:", err)
		cp.Close()
		return nil
	}
	info := ConnectInfo{
		Flag:   FLAG_OFFER,
		Source: id,
		SDP:    offer.SDP,
	}
	return &info
}

func (cp *ConnectionPair) MakeConnection(info ConnectInfo) {
	if err := cp.PeerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  info.SDP,
	}); err != nil {
		log.Println("make connection rtc error:", err)
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
		log.Println("Add cadidate!")

		err := cp.PeerConnection.AddICECandidate(*ca)
		if err != nil {
			log.Println(err, cp.ID, id)
		}
	} else {
		log.Println("Dismatched candidate id ", cp.ID, id)
	}
}

func (cp *ConnectionPair) IsRemoteDscripterSet() bool {
	if cp.PeerConnection.RemoteDescription() == nil {
		return false
	}
	return true
}
