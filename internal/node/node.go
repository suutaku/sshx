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
	"path"
	"sync"
	"time"
)

var (
	FLAG_UNKNOWN   = 0
	FLAG_CANDIDATE = 1
	FLAG_ANWER     = 2
	FLAG_OFFER     = 3

	//Connection pair types
	CP_TYPE_CLIENT = "_client"
	CP_TYPE_SERVER = "_server"
)

type ConnectInfo struct {
	Flag      int    `json:"flag"`
	Source    string `json:"source"`
	SDP       string `json:"sdp"`
	Candidate []byte `json:"candidate"`
	ID        int64  `json:"id"`
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
	ConnectionPairs map[string]*ConnectionPair
	connectionMux   sync.Mutex
	PendingCadidate map[string][]*webrtc.ICECandidateInit
	candidateMux    sync.Mutex
}

func NewNode(cnf *conf.Configure) *Node {
	return &Node{
		Configure:       cnf,
		ConnectionPairs: make(map[string]*ConnectionPair),
		PendingCadidate: make(map[string][]*webrtc.ICECandidateInit),
	}
}

func (node *Node) CloseConnections(key string) {
	node.candidateMux.Lock()
	delete(node.PendingCadidate, key)
	node.candidateMux.Unlock()

	node.connectionMux.Lock()
	defer node.connectionMux.Unlock()
	if node.ConnectionPairs[key] != nil {
		node.ConnectionPairs[key].Close()
		delete(node.ConnectionPairs, key)
		log.Println("Node close connection pair of ", key)
	}
}

func (node *Node) OpenConnections(key string, cType string, sc *net.Conn) chan int {
	node.CloseConnections(key + cType)
	node.connectionMux.Lock()
	defer node.connectionMux.Unlock()
	node.ConnectionPairs[key+cType] = NewConnectionPair(node.RTCConf, sc, cType)
	node.ConnectionPairs[key+cType].PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		node.SignalCandidate(key+cType, c)
	})
	return node.ConnectionPairs[key+cType].Exit
}

func (node *Node) SetConnectionPairID(key string, id int64) {
	if node.ConnectionPairs[key] != nil {
		node.ConnectionPairs[key].ID = id
	}
}

func (node *Node) AddCandidate(key string, ca *webrtc.ICECandidateInit, id int64) {
	node.candidateMux.Lock()
	if ca != nil {
		node.PendingCadidate[key] = append(node.PendingCadidate[key], ca)
	}
	if node.ConnectionPairs[key] != nil && node.ConnectionPairs[key].IsRemoteDscripterSet() {
		for _, v := range node.PendingCadidate[key] {
			node.ConnectionPairs[key].AddCandidate(v, id)
		}
		delete(node.PendingCadidate, key)
	}
	node.candidateMux.Unlock()
}

func (node *Node) SignalCandidate(addr string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	if node.ConnectionPairs[addr] == nil {
		return
	}
	info := ConnectInfo{
		Flag:      FLAG_CANDIDATE,
		Source:    node.ID,
		Candidate: []byte(c.ToJSON().Candidate),
		ID:        node.ConnectionPairs[addr].ID,
	}
	node.push(info, addr)
	log.Println("Push candidate!")

}

func (node *Node) Start(ctx context.Context) {

	// if node is a full node, listen as a "server"
	if node.FullNode {
		log.Println("start as a full node")
		go node.Serve(ctx)
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
			go node.Connect(ctx, sock)
		}
	}()
}

func (node *Node) Anwser(info ConnectInfo) *ConnectInfo {
	ssh, err := net.Dial("tcp", node.LocalSSHAddr)
	if err != nil {
		log.Println("ssh dial filed:", err)
		node.CloseConnections(info.Source + CP_TYPE_CLIENT)
		return nil
	}
	node.OpenConnections(info.Source, CP_TYPE_CLIENT, &ssh)
	node.SetConnectionPairID(info.Source+CP_TYPE_CLIENT, info.ID)
	return node.ConnectionPairs[info.Source+CP_TYPE_CLIENT].Anwser(info, node.ID)
}

func (node *Node) Offer(key string) *ConnectInfo {
	info := node.ConnectionPairs[key].Offer(node.ID)
	info.ID = node.ConnectionPairs[key].ID
	return info
}

func (node *Node) Serve(ctx context.Context) {
	for v := range node.pull(ctx, node.ID+CP_TYPE_SERVER) {
		log.Printf("info: %#v", v)
		switch v.Flag {
		case FLAG_OFFER:
			tmp := node.Anwser(v)
			if tmp != nil {
				node.push(*tmp, v.Source+CP_TYPE_CLIENT)
			}
		case FLAG_CANDIDATE:
			node.AddCandidate(v.Source+CP_TYPE_CLIENT, &webrtc.ICECandidateInit{Candidate: string(v.Candidate)}, v.ID)
		case FLAG_ANWER:
			log.Println("Bad connection info")
		case FLAG_UNKNOWN:
			log.Println("Unknown connection info")
		}
	}
}

func (node *Node) Connect(ctx context.Context, sock net.Conn) {
	key := node.Key
	ch := node.OpenConnections(key, CP_TYPE_SERVER, &sock)
	info := node.Offer(key + CP_TYPE_SERVER)
	node.push(*info, key+CP_TYPE_SERVER)
	ctxSub, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(sub context.Context) {
		for {
			select {
			case v := <-node.pull(sub, node.ID+CP_TYPE_CLIENT):
				switch v.Flag {
				case FLAG_OFFER:
					log.Println("Bad connection info")
				case FLAG_CANDIDATE:
					node.AddCandidate(v.Source+CP_TYPE_SERVER, &webrtc.ICECandidateInit{Candidate: string(v.Candidate)}, v.ID)
				case FLAG_ANWER:
					node.ConnectionPairs[key+CP_TYPE_SERVER].MakeConnection(v)
					break
				case FLAG_UNKNOWN:
					log.Println("Unknown connection info")
				}
			case <-sub.Done():
				log.Println("client loop exit 1")
				return
			}
		}
	}(ctxSub)
	<-ch
	close(ch)
	log.Println("client loop exit 2")
}

func (node *Node) push(info ConnectInfo, target string) error {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(info); err != nil {
		return err
	}
	resp, err := http.Post(node.SignalingServerAddr+
		path.Join("/", "push", target), "application/json", buf)
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
			req, err := http.NewRequest("GET", node.SignalingServerAddr+
				path.Join("/", "pull", id), nil)
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
			if len(info.Source) < 0 ||
				(info.Flag != FLAG_CANDIDATE && len(info.SDP) < 0) {
				log.Println("sdp is emtpy with flag 0")
			} else {
				ch <- info
			}
		}
	}()
	return ch
}
