package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/proto"
)

type VNCProxy struct {
	h5ClientStaticPath string
	proxyAddr          string
	node               *Node
}

func NewVNCProxy(node *Node) *VNCProxy {
	return &VNCProxy{
		h5ClientStaticPath: node.ConfManager.Conf.VNCStaticPath,
		proxyAddr:          node.ConfManager.Conf.GuacListenAddr,
		node:               node,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (vp *VNCProxy) Start() {
	r := mux.NewRouter()
	s := http.StripPrefix("/", http.FileServer(http.Dir(vp.h5ClientStaticPath)))
	r.PathPrefix("/").Handler(s)
	http.Handle("/", r)
	http.HandleFunc("/conf", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.Error(err)
			return
		}
		defer conn.Close()
		req := proto.SetConfigRequest{}
		_, buf, err := conn.ReadMessage()
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Debug(buf)
		err = jsonpb.UnmarshalString(string(buf), &req)
		if err != nil {
			logrus.Error(err)
			return
		}
		innerCon, err := net.DialTimeout("tcp", vp.node.ConfManager.Conf.LocalListenAddr, time.Second)
		if err != nil {
			logrus.Error(err)
			return
		}
		defer innerCon.Close()
		preReq := proto.ConnectRequest{
			Type: conf.OPT_TYPE_SET_CONFIG,
		}
		b, _ := preReq.Marshal()
		if err != nil {
			logrus.Error(err)
			return
		}
		innerCon.Write(b)
		innerCon.Read(b)
		b, _ = req.Marshal()
		innerCon.Write(b)
		// innerCon.Read(b)
		// w.Write(b)

	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		req, err := vp.ParseConnectionRequest(r)
		if err != nil {
			logrus.Error(err)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.Error(err)
			return
		}
		vp.node.Connect(context.TODO(), conn.UnderlyingConn(), req)
	})

	logrus.Info("start vnc proxy at: ", vp.proxyAddr)
	http.ListenAndServe(vp.proxyAddr, nil)
}

func (vp *VNCProxy) ParseConnectionRequest(r *http.Request) (proto.ConnectRequest, error) {
	deviceId := r.URL.Query()["device"]
	if len(deviceId) < 1 {
		return proto.ConnectRequest{}, fmt.Errorf("invalid device id")
	}
	req := proto.ConnectRequest{
		Host:      deviceId[0],
		Type:      conf.OPT_TYPE_START_VNC,
		Timestamp: time.Now().Unix(),
	}
	return req, nil
}
