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
	ConnectionPairs map[int64]*ConnectionPair
	connectionMux   sync.Mutex
	PendingCadidate map[int64][]*webrtc.ICECandidateInit
	candidateMux    sync.Mutex
}

func NewNode(cnf *conf.Configure) *Node {
	return &Node{
		Configure:       cnf,
		ConnectionPairs: make(map[int64]*ConnectionPair),
		PendingCadidate: make(map[int64][]*webrtc.ICECandidateInit),
	}
}

func (node *Node) CloseConnections(key int64) {
	log.Println("close start")
	node.candidateMux.Lock()
	delete(node.PendingCadidate, key)
	node.candidateMux.Unlock()
	log.Println("cadiate close done")
	if node.ConnectionPairs[key] != nil {
		log.Println("pair start")
		node.connectionMux.Lock()
		node.ConnectionPairs[key].Close()
		close(node.ConnectionPairs[key].Exit)
		delete(node.ConnectionPairs, key)
		node.ConnectionPairs[key] = nil
		node.connectionMux.Unlock()
		log.Println("Node close connection pair of ", key)
	}
}

func (node *Node) OpenConnections(sc *net.Conn, cType string, connID int64) *ConnectionPair {
	tmp := NewConnectionPair(node.RTCConf, sc, cType, connID)
	if tmp == nil {
		return nil
	}
	node.connectionMux.Lock()
	node.ConnectionPairs[tmp.ID] = tmp
	node.connectionMux.Unlock()
	return tmp
}

func (node *Node) SetConnectionPairID(id int64, newID int64) {
	log.Println("Server befor change id ", node.ConnectionPairs[id].ID)
	node.connectionMux.Lock()
	node.ConnectionPairs[newID] = node.ConnectionPairs[id]
	node.ConnectionPairs[newID].ID = newID
	node.connectionMux.Unlock()
	log.Println("Server after id ", node.ConnectionPairs[newID].ID)
}

func (node *Node) AddCandidate(ca *webrtc.ICECandidateInit, id int64) {
	node.candidateMux.Lock()
	if ca != nil {
		node.PendingCadidate[id] = append(node.PendingCadidate[id], ca)
	}
	if node.ConnectionPairs[id] != nil && node.ConnectionPairs[id].IsRemoteDscripterSet() {
		for _, v := range node.PendingCadidate[id] {
			node.ConnectionPairs[id].AddCandidate(v)
		}
		delete(node.PendingCadidate, id)
		log.Println("Add Candidate info xxx", id)
	}
	node.candidateMux.Unlock()
}

func (node *Node) SignalCandidate(id int64, addr string, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	if node.ConnectionPairs[id] == nil {
		log.Println("Connection not exists !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
		return
	}
	info := ConnectInfo{
		Flag:      FLAG_CANDIDATE,
		Source:    node.ID,
		Candidate: []byte(c.ToJSON().Candidate),
		ID:        id,
	}
	node.push(info, addr)
	log.Println("Push candidate", info.ID, addr)

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
		node.CloseConnections(info.ID)
		return nil
	}
	conn := node.OpenConnections(&ssh, CP_TYPE_CLIENT, info.ID)
	log.Println("Server create a connection with id ", conn.ID)
	conn.PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		node.SignalCandidate(info.ID, info.Source+CP_TYPE_CLIENT, c)
	})
	return conn.Anwser(info, node.ID)
}

func (node *Node) Offer(id int64) *ConnectInfo {
	info := node.ConnectionPairs[id].Offer(id, node.ID)
	return info
}

func (node *Node) Serve(ctx context.Context) {
	for {
		select {
		case v := <-node.pull(ctx, node.ID+CP_TYPE_SERVER):
			switch v.Flag {
			case FLAG_OFFER:
				log.Println("Offer info", v.ID)
				tmp := node.Anwser(v)
				if tmp != nil {
					node.push(*tmp, v.Source+CP_TYPE_CLIENT)
					log.Println("Push anwsesr ", v.Source+CP_TYPE_CLIENT, v.ID)
				}
			case FLAG_CANDIDATE:
				log.Println("Candidate info", v.ID)
				node.AddCandidate(&webrtc.ICECandidateInit{Candidate: string(v.Candidate)}, v.ID)
			case FLAG_ANWER:
				log.Println("Bad connection info")
			case FLAG_UNKNOWN:
				log.Println("Unknown connection info")
			}
		case <-ctx.Done():
			log.Println("Server canceled")
			return
		}
	}
}

func (node *Node) Connect(ctx context.Context, sock net.Conn) {
	key := node.Configure.Key
	if key == "" {
		log.Println("target id is empty")
		return
	}
	conn := node.OpenConnections(&sock, CP_TYPE_SERVER, 0)
	node.ConnectionPairs[conn.ID].PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		node.SignalCandidate(conn.ID, key+CP_TYPE_SERVER, c)
	})
	info := node.Offer(conn.ID)
	node.push(*info, key+CP_TYPE_SERVER)
	log.Println("Push offer ", info.ID, info.Source)
	sub, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(subCtx context.Context) {
		log.Println("New go runtinue")
		ctxT, cancelT := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelT()
		for {
			select {
			case v := <-node.pull(ctx, node.ID+CP_TYPE_CLIENT):
				switch v.Flag {
				case FLAG_OFFER:
					log.Println("Bad connection info", v.ID)
				case FLAG_CANDIDATE:
					log.Println("Candidate info", v.ID)
					node.AddCandidate(&webrtc.ICECandidateInit{Candidate: string(v.Candidate)}, v.ID)
				case FLAG_ANWER:
					log.Println("Anwser info ", v.ID)
					node.ConnectionPairs[v.ID].MakeConnection(v)
				case FLAG_UNKNOWN:
					log.Println("Unknown connection info", v.ID)
				}
			case <-subCtx.Done():
				log.Println("Connected, loop exit!")
				return
			case <-ctx.Done():
				log.Println("Canceled, loop exit 2!")
				return
			case <-ctxT.Done():
				log.Println("Timeout, loop exit 3!")
				//node.CloseConnections(v.ID)
				return
			}
		}
	}(sub)
	// var n = 5
	// for {
	// 	node.AddCandidate(nil, id)
	// 	time.Sleep(1 * time.Second)
	// 	n--
	// 	if n < 0 {
	// 		return
	// 	}
	// }
	<-conn.Exit
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
