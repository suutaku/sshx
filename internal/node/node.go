package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/proto"
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
	Timestamp int64
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
	pm              *ProxyManager
}

func NewNode(cnf *conf.Configure) *Node {
	return &Node{
		Configure:       cnf,
		ConnectionPairs: make(map[string]*ConnectionPair),
		PendingCadidate: make(map[string][]*webrtc.ICECandidateInit),
		pm:              NewProxyManager(),
	}
}

func (node *Node) CloseConnections(key string) {
	node.candidateMux.Lock()
	delete(node.PendingCadidate, key)
	node.candidateMux.Unlock()

	node.connectionMux.Lock()
	defer node.connectionMux.Unlock()
	if node.ConnectionPairs[key] != nil {
		close(node.ConnectionPairs[key].Exit)
		node.ConnectionPairs[key].Close()
		delete(node.ConnectionPairs, key)
		logrus.Debug("Node close connection pair of ", key)
	}
}

func (node *Node) OpenConnections(target proto.ConnectRequest, cType string, sc *net.Conn) chan int {
	key := fmt.Sprintf("%s%d", target.Host, target.Timestamp)
	logrus.Debug("key ", key)
	node.connectionMux.Lock()
	logrus.Debug("query lock ", key)
	defer node.connectionMux.Unlock()
	node.ConnectionPairs[key] = NewConnectionPair(node.RTCConf, sc, cType)
	node.ConnectionPairs[key].PeerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		node.SignalCandidate(target, c)
	})
	logrus.Debug("return close channel ", key)
	return node.ConnectionPairs[key].Exit
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

func (node *Node) SignalCandidate(target proto.ConnectRequest, c *webrtc.ICECandidate) {
	if c == nil {
		return
	}
	key := fmt.Sprintf("%s%d", target.Host, target.Timestamp)
	if node.ConnectionPairs[key] == nil {
		return
	}
	info := ConnectInfo{
		Flag:      FLAG_CANDIDATE,
		Source:    node.ID,
		Candidate: []byte(c.ToJSON().Candidate),
		ID:        node.ConnectionPairs[key].ID,
		Timestamp: target.Timestamp,
	}
	node.push(info, target.Host)
	logrus.Debug("Push candidate to ", target.Host, "!")

}

func (node *Node) Start(ctx context.Context) {

	// if node is a full node, listen as a "server"
	go node.Serve(ctx)

	// listen as a "client"
	l, err := net.Listen("tcp", node.LocalListenAddr)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	logrus.Info("local main service listenning on:", node.LocalListenAddr)
	go func() {
		for {
			sock, err := l.Accept()
			if err != nil {
				sock.Close()
				continue
			}
			// read id and do connect
			buf := make([]byte, 512)
			n, err := sock.Read(buf)
			if err != nil {
				logrus.Error(err)
				sock.Close()
				continue
			}
			_, err = sock.Write([]byte("ok"))
			if err != nil {
				logrus.Error(err)
				sock.Close()
				continue
			} else {
				req := proto.ConnectRequest{}
				err = req.Unmarshal(buf[:n])
				if err != nil {
					logrus.Error(err, string(buf[:n]))
					sock.Close()
					continue
				}
				switch req.Type {
				case conf.TYPE_CONNECTION:
					logrus.Debug("make a connection to ", req.Host)
					go node.Connect(ctx, sock, req)
				case conf.TYPE_START_PROXY:
					logrus.Debug("start a proxy to ", req.Host)
					conf.ClearKnownHosts("127.0.0.1")
					subCtx, cancl := context.WithCancel(context.Background())
					go node.Proxy(subCtx, req)
					rep := ProxyRepo{
						ProxyInfo: proto.ProxyInfo{
							Host:             req.Host,
							Port:             req.Port,
							ProxyPort:        req.ProxyPort,
							StartTime:        time.Now().Unix(),
							X11:              req.X11,
							ConnectionNumber: 0,
						},
						ConnetionKey: fmt.Sprintf("%s%d", req.Host, req.Timestamp),
						cancel:       &cancl,
					}
					node.pm.AddProxy(&rep)
				case conf.TYPE_CLOSE_CONNECTION:
					logrus.Debug("close connection to ", req.Host)
					go node.CloseConnections(fmt.Sprintf("%s%d", req.Host, req.Timestamp))
					sock.Close()
				case conf.TYPE_STOP_PROXY:
					list := node.pm.GetConnectionKeys(req.Host)
					logrus.Debug("stop proxy to %v", list)
					for _, v := range list {
						node.CloseConnections(v)
					}
					node.pm.RemoveProxy(req.Host)
					sock.Close()
				case conf.TYPE_PROXY_LIST:
					list := node.pm.GetProxyInfos(req.Host)
					b, err := list.Marshal()
					if err != nil {
						logrus.Error(err)
					}
					sock.Write(b)
				case conf.TYPE_START_VNC:
					logrus.Debug("start vnc request come")
				case conf.TYPE_STOP_VNC:
					logrus.Debug("stop vnc request come")

				}
			}
		}
	}()
}

func (node *Node) Anwser(info ConnectInfo) *ConnectInfo {
	ssh, err := net.Dial("tcp", node.LocalSSHAddr)
	if err != nil {
		logrus.Error("ssh dial filed:", err)
		node.CloseConnections(info.Source)
		return nil
	}
	req := proto.ConnectRequest{
		Host:      info.Source,
		Timestamp: info.Timestamp,
	}
	key := fmt.Sprintf("%s%d", req.Host, req.Timestamp)
	node.OpenConnections(req, CP_TYPE_CLIENT, &ssh)
	node.SetConnectionPairID(key, info.ID)
	return node.ConnectionPairs[key].Anwser(info, node.ID)
}

func (node *Node) Offer(req proto.ConnectRequest) *ConnectInfo {
	key := fmt.Sprintf("%s%d", req.Host, req.Timestamp)
	info := node.ConnectionPairs[key].Offer(node.ID)
	info.Timestamp = req.Timestamp
	info.ID = node.ConnectionPairs[key].ID
	return info
}

func (node *Node) Serve(ctx context.Context) {
	logrus.Println("start sshx daemon")
	for {
		select {
		case v := <-node.pull(ctx):
			switch v.Flag {
			case FLAG_OFFER:
				tmp := node.Anwser(v)
				if tmp != nil {
					node.push(*tmp, v.Source)
				}
			case FLAG_CANDIDATE:
				node.AddCandidate(fmt.Sprintf("%s%d", v.Source, v.Timestamp), &webrtc.ICECandidateInit{Candidate: string(v.Candidate)}, v.ID)
			case FLAG_ANWER:
				node.ConnectionPairs[fmt.Sprintf("%s%d", v.Source, v.Timestamp)].MakeConnection(v)
			case FLAG_UNKNOWN:
				logrus.Error("unknown connection info")
			}
		case <-ctx.Done():
			logrus.Println("stop sshx daemon")

		}
	}
}

func (node *Node) Connect(ctx context.Context, sock net.Conn, target proto.ConnectRequest) {

	ch := node.OpenConnections(target, CP_TYPE_SERVER, &sock)
	info := node.Offer(target)
	info.Timestamp = target.Timestamp
	err := node.push(*info, target.Host)
	if err != nil {
		logrus.Error(err)
	}
	logrus.Debug("watting connection abord")
	logrus.Debug("end of connection option", <-ch)
}

func (node *Node) push(info ConnectInfo, target string) error {
	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(info); err != nil {
		return err
	}
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Post(node.SignalingServerAddr+
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

func (node *Node) pull(ctx context.Context) <-chan ConnectInfo {
	ch := make(chan ConnectInfo, 1)
	for {
		client := http.Client{
			Timeout: 30 * time.Second,
		}
		res, err := client.Get(node.SignalingServerAddr +
			path.Join("/", "pull", node.ID))
		if err != nil {
			if ctx.Err() == context.Canceled {
				return nil
			}
			continue
		}
		defer res.Body.Close()
		var info ConnectInfo
		if err = json.NewDecoder(res.Body).Decode(&info); err != nil {
			if err == io.EOF {
				continue
			}
			if ctx.Err() == context.Canceled {
				return nil
			}
			continue
		}
		ch <- info
		logrus.Debug("pull success")
		return ch
	}
}
