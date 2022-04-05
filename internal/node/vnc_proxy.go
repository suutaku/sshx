package node

import (
	"context"
	"fmt"
	"net/http"
	"time"

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
		h5ClientStaticPath: "/Users/john/Desktop/work/sshx/http",
		proxyAddr:          "127.0.0.1:80",
		node:               node,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (vp *VNCProxy) Start() {
	r := mux.NewRouter()
	s := http.StripPrefix("/", http.FileServer(http.Dir(vp.h5ClientStaticPath)))
	r.PathPrefix("/").Handler(s)
	http.Handle("/", r)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		logrus.Info("ws connection comes", r.URL)
		req, err := vp.ParseConnectionRequest(r)
		if err != nil {
			logrus.Error(err)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.Println(err)
			return
		}

		vp.node.Connect(context.TODO(), conn.UnderlyingConn(), req)
		select {}
		logrus.Info("ws done", r.URL)
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
		Type:      conf.TYPE_START_VNC,
		Timestamp: time.Now().Unix(),
	}
	return req, nil
}
