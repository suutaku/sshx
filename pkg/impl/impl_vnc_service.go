package impl

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/types"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type VNCService struct {
	BaseImpl
	VNCPort       int32
	Running       bool
	VNCStaticPath string
	httpServer    *http.Server
}

func NewVNCService(port int32) *VNCService {
	return &VNCService{
		VNCPort: port,
	}
}

func (vnc *VNCService) Code() int32 {
	return types.APP_TYPE_VNC_SERVICE
}

func (vnc *VNCService) Preper() error {
	return nil
}

func (vnc *VNCService) Dial() error {
	vnc.Running = true
	cm := conf.NewConfManager("")
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cm.Conf.LocalHTTPPort)}
	r := mux.NewRouter()
	s := http.StripPrefix("/", http.FileServer(http.Dir(cm.Conf.VNCStaticPath)))
	r.PathPrefix("/").Handler(s)
	http.Handle("/", r)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		deviceId := r.URL.Query()["device"]
		logrus.Debug(deviceId)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logrus.Error(err)
			return
		}
		defer conn.Close()

		imp := NewVNC(deviceId[0])
		err = imp.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		imp.SetParentId(vnc.PairId())
		sender := NewSender(imp, types.OPTION_TYPE_UP)
		if sender == nil {
			logrus.Error("cannot create sender")
			return
		}
		inConn, err := sender.Send()
		if err != nil {
			logrus.Error(err)
			return
		}
		defer conn.Close()
		// defer func() {
		// 	conn.Close()
		// 	closeSender := NewSender(imp, types.OPTION_TYPE_DOWN)
		// 	closeSender.PairId = sender.PairId
		// 	closeSender.SendDetach()
		// }()
		underConn := conn.UnderlyingConn()
		utils.PipeWR(inConn, underConn, inConn, underConn)
		logrus.Debug("end of gorutine")

	})
	logrus.Info("servce http at port ", cm.Conf.LocalHTTPPort)
	vnc.httpServer = srv
	srv.ListenAndServe()
	return nil
}

func (vnc *VNCService) Response() error {
	return nil
}

func (vnc *VNCService) Close() {
	if vnc.httpServer != nil {
		vnc.httpServer.Shutdown(context.TODO())
	}
}
