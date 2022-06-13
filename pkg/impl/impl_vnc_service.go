package impl

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	vncconf "github.com/suutaku/go-vnc/pkg/config"
	vncgo "github.com/suutaku/go-vnc/pkg/vnc"
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
	Running       bool
	VNCStaticPath string
	VNCConf       *vncconf.Configure
	httpServer    *http.Server
	vncServer     *vncgo.VNC
}

func NewVNCService(conf *vncconf.Configure) *VNCService {
	return &VNCService{
		VNCConf: conf,
	}
}

func (vnc *VNCService) Code() int32 {
	return types.APP_TYPE_VNC_SERVICE
}

func (vnc *VNCService) serviceIsRuning(port int32) bool {
	res, err := http.Head(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err == nil && res.StatusCode == 200 {
		logrus.Warn("vnc server was already runing")
		return true
	}
	return false
}

func (vnc *VNCService) Dial() error {
	vnc.Running = true
	cm := conf.NewConfManager("")
	if vnc.VNCConf == nil {
		vnc.VNCConf = &cm.Conf.VNCConf
	}
	if vnc.serviceIsRuning(cm.Conf.LocalHTTPPort) {
		return fmt.Errorf("vnc service was already running")
	}
	r := mux.NewRouter()
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(cm.Conf.VNCStaticPath)))
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
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
		underConn := conn.UnderlyingConn()
		utils.PipeWR(inConn, underConn, inConn, underConn)
		logrus.Debug("end of gorutine")

	})
	logrus.Info("servce http at port ", cm.Conf.LocalHTTPPort)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cm.Conf.LocalHTTPPort), Handler: r}
	vnc.httpServer = srv
	vnc.vncServer = vncgo.NewVNC(context.Background(), *vnc.VNCConf)
	go vnc.vncServer.Start()
	srv.ListenAndServe()
	return nil
}

func (vnc *VNCService) Response() error {
	return nil
}

func (vnc *VNCService) Close() {
	if vnc.vncServer != nil {
		logrus.Debug("close vnc server")
		vnc.vncServer.Close()
	}
	if vnc.httpServer != nil {
		logrus.Debug("close http server")
		vnc.httpServer.Shutdown(context.TODO())
	}
}
