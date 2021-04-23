package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/webrtc/v3"
	"github.com/suutaku/sshx/internal/conf"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"time"
)

type ConnectInfo struct {
	Source string `json:"source"`
	SDP    string `json:"sdp"`
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
}

func NewNode(path string) *Node {
	return &Node{
		Configure:         conf.NewConfigure(path),
		ConnectionManager: NewConnectionManager(),
	}
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
		pc, err := webrtc.NewPeerConnection(node.RTCConf)
		if err != nil {
			log.Println("rtc error:", err)
			continue
		}
		ssh, err := net.Dial("tcp", node.LocalSSHAddr)
		if err != nil {
			log.Println("ssh dial filed:", err)
			pc.Close()
			continue
		}
		pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
			log.Print("pc ice state change:", state)
			if state == ice.ConnectionStateDisconnected {
				pc.Close()
				ssh.Close()
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
			continue
		}
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			log.Println("rtc error:", err)
			pc.Close()
			ssh.Close()
			continue
		}

		v.SDP = answer.SDP
		if err := node.push(v, node.ID); err != nil {
			log.Println("rtc error:", err)
			pc.Close()
			ssh.Close()
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
	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Print("pc ice state change:", state)
	})
	dc, err := pc.CreateDataChannel("data", nil)
	if err != nil {
		log.Println("create dc failed:", err)
		pc.Close()
		return
	}
	//dc.Lock()
	dc.OnOpen(func() {
		log.Println("wrap sock to dc")
		io.Copy(&sendWrap{dc}, sock)
		pc.Close()
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		_, err := sock.Write(msg.Data)
		if err != nil {
			log.Println("sock write failed:", err)
			pc.Close()
			return
		}
	})
	//dc.Unlock()
	//log.Print("DataChannel:", dc)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		//pull with myself ID
		for v := range node.pull(ctx, key) {
			log.Printf("info: %#v", v)
			if err := pc.SetRemoteDescription(webrtc.SessionDescription{
				Type: webrtc.SDPTypeAnswer,
				SDP:  v.SDP,
			}); err != nil {
				log.Println("rtc error:", err)
				pc.Close()
				return
			}
			return
		}
	}()
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Println("create offer error:", err)
		pc.Close()
		return
	}
	if err := node.push(ConnectInfo{Source: node.ID, SDP: offer.SDP}, key); err != nil {
		log.Println("push error:", err)
		pc.Close()
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
			if len(info.Source) > 0 && len(info.SDP) > 0 {
				ch <- info
			}
		}
	}()
	return ch
}
