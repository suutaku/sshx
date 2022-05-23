package impl

import (
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
	wsconn        *websocket.Conn
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
		underConn := conn.UnderlyingConn()
		utils.PipeWR(inConn, underConn, inConn, underConn)
		logrus.Debug("end of gorutine")

	})
	logrus.Info("servce http at port ", cm.Conf.LocalHTTPPort)
	http.ListenAndServe(fmt.Sprintf(":%d", cm.Conf.LocalHTTPPort), nil)
	return nil
}

func (vnc *VNCService) Response() error {
	return nil
}

func (vnc *VNCService) Close() {
	vnc.BaseImpl.Close()
	if vnc.wsconn != nil {
		vnc.wsconn.Close()
	}
}
