package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"github.com/suutaku/sshx/internal/conf"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

var (
	FLAG_CANDIDATE = 1
)

type ConnectInfo struct {
	Flag      int    `json:"flag"`
	Source    string `json:"source"`
	SDP       string `json:"sdp"`
	Candidate []byte `json:"candidate"`
}

type sendWrap struct {
	*webrtc.DataChannel
}

func (s *sendWrap) Write(b []byte) (int, error) {
	err := s.DataChannel.Send(b)
	return len(b), err
}

type Node struct {
	*conf.Configure
	*ConnectionManager
	PeerConnections   map[string]*webrtc.PeerConnection
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
}

func NewNode(path string) *Node {
	return &Node{
		Configure:         conf.NewConfigure(path),
		ConnectionManager: NewConnectionManager(),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
		PeerConnections:   make(map[string]*webrtc.PeerConnection),
	}
}

func (node *Node) signalCandidate(addr string, c *webrtc.ICECandidate) {
	log.Println("Push candidate!")
	info := ConnectInfo{
		Flag:      FLAG_CANDIDATE,
		Source:    node.ID,
		Candidate: []byte(c.ToJSON().Candidate),
	}
	node.push(info, addr)
}

func (node *Node) Start(ctx context.Context) {

	// if node is a full node, listen as a "server"
	if node.FullNode {
		log.Println("start as a full node")
		go node.serve(ctx)
	}

	// listen as a "client"
	l, err := net.Listen("tcp", node.LocalListenAddr)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("listen:", node.LocalListenAddr)
	go func() {
		for {
			sock, err := l.Accept()
			if err != nil {
				log.Println(err)
				continue
			}
			sshxKey := os.Getenv("SSHX_KEY")
			if sshxKey == "" {
				log.Println("SSHX_KEY (target id) is empty")
				continue
			}
			go node.connect(ctx, sshxKey, sock)
		}
	}()
}

func (node *Node) serve(ctx context.Context) {
	log.Println("server started")
	// pull with myself ID
	for v := range node.pull(ctx, node.ID) {
		log.Printf("info: %#v", v)
		if v.Flag == FLAG_CANDIDATE && node.PeerConnections[v.Source] != nil {
			if candidateErr := node.PeerConnections[v.Source].AddICECandidate(webrtc.ICECandidateInit{Candidate: string(v.Candidate)}); candidateErr != nil {
				log.Println(candidateErr)
			}
			log.Println("Add candidate!!")
			continue
		}
		pc, err := webrtc.NewPeerConnection(node.RTCConf)
		if err != nil {
			log.Println("rtc error:", err)
			continue
		}
		node.PeerConnections[v.Source] = pc
		pc.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c == nil {
				return
			}

			node.candidatesMux.Lock()
			defer node.candidatesMux.Unlock()

			desc := pc.RemoteDescription()
			if desc == nil {
				node.pendingCandidates = append(node.pendingCandidates, c)
			} else {
				node.signalCandidate(v.Source, c)
			}
		})
		ssh, err := net.Dial("tcp", node.LocalSSHAddr)
		if err != nil {
			log.Println("ssh dial filed:", err)
			pc.Close()
			delete(node.PeerConnections, v.Source)
			continue
		}
		pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
			log.Print("pc ice state change:", state)
			if state == webrtc.ICEConnectionStateDisconnected ||
				state == webrtc.ICEConnectionStateFailed ||
				state == webrtc.ICEConnectionStateClosed {
				pc.Close()
				ssh.Close()
				delete(node.PeerConnections, v.Source)
			}
		})
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			log.Print("pc connection state change:", state)
			if state == webrtc.PeerConnectionStateFailed ||
				state == webrtc.PeerConnectionStateDisconnected ||
				state == webrtc.PeerConnectionStateClosed {
				pc.Close()
				ssh.Close()
				delete(node.PeerConnections, v.Source)
			}
		})

		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			//dc.Lock()
			dc.OnOpen(func() {
				log.Println("wrap to ssh dc")
				io.Copy(&sendWrap{dc}, ssh)
			})
			dc.OnMessage(func(msg webrtc.DataChannelMessage) {
				_, err := ssh.Write(msg.Data)
				if err != nil {
					log.Println("sock write failed:", err)
					pc.Close()
					delete(node.PeerConnections, v.Source)
					return
				}
			})
			//dc.Unlock()
		})
		if err := pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  v.SDP,
		}); err != nil {
			log.Println("rtc error:", err)
			pc.Close()
			ssh.Close()
			delete(node.PeerConnections, v.Source)
			continue
		}
		node.candidatesMux.Lock()
		for _, c := range node.pendingCandidates {
			node.signalCandidate(v.Source, c)
		}
		node.candidatesMux.Unlock()
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			log.Println("rtc error:", err)
			pc.Close()
			ssh.Close()
			delete(node.PeerConnections, v.Source)
			continue
		}

		err = pc.SetLocalDescription(answer)
		if err != nil {
			ssh.Close()
			continue
		}

		v.SDP = answer.SDP
		target := v.Source
		v.Source = node.ID
		if err := node.push(v, target); err != nil {
			log.Println("rtc error:", err)
			pc.Close()
			ssh.Close()
			delete(node.PeerConnections, v.Source)
			continue
		}

	}
}

func (node *Node) connect(ctx context.Context, key string, sock net.Conn) {

	pc, err := webrtc.NewPeerConnection(node.RTCConf)
	if err != nil {
		log.Println("rtc error:", err)
		return
	}
	node.PeerConnections[key] = pc
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		node.candidatesMux.Lock()
		defer node.candidatesMux.Unlock()

		desc := pc.RemoteDescription()
		if desc == nil {
			node.pendingCandidates = append(node.pendingCandidates, c)
		} else {
			node.signalCandidate(key, c)
		}
	})
	dc, err := pc.CreateDataChannel("data", nil)
	if err != nil {
		log.Println("create dc failed:", err)
		pc.Close()
		sock.Close()
		delete(node.PeerConnections, key)
		return
	}
	//dc.Lock()
	dc.OnOpen(func() {
		log.Println("wrap sock to dc")
		io.Copy(&sendWrap{dc}, sock)
		pc.Close()
		sock.Close()
		delete(node.PeerConnections, key)
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		_, err := sock.Write(msg.Data)
		if err != nil {
			log.Println("sock write failed:", err)
			pc.Close()
			sock.Close()
			delete(node.PeerConnections, key)
			return
		}
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Print("pc connection state change:", state)
		if state == webrtc.PeerConnectionStateDisconnected ||
			state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed {
			pc.Close()
			sock.Close()
			delete(node.PeerConnections, key)
			log.Println("start cancel context !")
		}
	})
	//dc.Unlock()
	//log.Print("DataChannel:", dc)
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func(ctx context.Context, cancel context.CancelFunc) {
		pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
			log.Print("pc ice state change:", state)
			if state == webrtc.ICEConnectionStateDisconnected ||
				state == webrtc.ICEConnectionStateFailed ||
				state == webrtc.ICEConnectionStateClosed {
				pc.Close()
				sock.Close()
				delete(node.PeerConnections, key)
				log.Println("start cancel context !")
				cancel()
			}
		})
		//pull with myself ID
		for v := range node.pull(ctx, node.ID) {
			log.Printf("info: %#v", v)
			if v.Flag == FLAG_CANDIDATE && node.PeerConnections[key] != nil {
				if candidateErr := node.PeerConnections[key].AddICECandidate(webrtc.ICECandidateInit{Candidate: string(v.Candidate)}); candidateErr != nil {
					log.Println(candidateErr)
				}
				continue
			} else {
				if err := pc.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.SDPTypeAnswer,
					SDP:  v.SDP,
				}); err != nil {
					log.Println("rtc error:", err)
					pc.Close()
					sock.Close()
					delete(node.PeerConnections, key)
					return
				}
				node.candidatesMux.Lock()
				for _, c := range node.pendingCandidates {
					node.signalCandidate(key, c)
				}
				node.candidatesMux.Unlock()
			}
		}
	}(ctx2, cancel2)
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Println("create offer error:", err)
		pc.Close()
		sock.Close()
		delete(node.PeerConnections, key)
		return
	}
	if err = pc.SetLocalDescription(offer); err != nil {
		pc.Close()
		sock.Close()
		delete(node.PeerConnections, key)
		return
	}
	if err := node.push(ConnectInfo{Source: node.ID, SDP: offer.SDP}, key); err != nil {
		log.Println("push error:", err)
		pc.Close()
		sock.Close()
		delete(node.PeerConnections, key)
		return
	}
}

func (node *Node) push(info ConnectInfo, target string) error {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(info); err != nil {
		return err
	}
	resp, err := http.Post(node.SignalingServerAddr+path.Join("/", "push", target), "application/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http failed")
	}
	return nil
}

func (node *Node) pull(ctx context.Context, id string) <-chan ConnectInfo {
	ch := make(chan ConnectInfo)
	var retry time.Duration
	go func() {
		faild := func() {
			if retry < 10 {
				retry++
			}
			time.Sleep(retry * time.Second)
		}
		defer close(ch)
		for {
			req, err := http.NewRequest("GET", node.SignalingServerAddr+path.Join("/", "pull", id), nil)
			if err != nil {
				if ctx.Err() == context.Canceled {
					return
				}
				log.Println("get failed:", err)
				faild()
				continue
			}
			req = req.WithContext(ctx)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				if ctx.Err() == context.Canceled {
					return
				}
				log.Println("get failed:", err)
				faild()
				continue
			}
			defer res.Body.Close()
			retry = time.Duration(0)
			var info ConnectInfo
			if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
				if err == io.EOF {
					continue
				}
				if ctx.Err() == context.Canceled {
					return
				}
				log.Println("get failed:", err)
				faild()
				continue
			}
			if len(info.Source) < 0 || (info.Flag != FLAG_CANDIDATE && len(info.SDP) < 0) {

			} else {
				ch <- info
			}
		}
	}()
	return ch
}
