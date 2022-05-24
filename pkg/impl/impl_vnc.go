package impl

import (
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/types"
)

type VNC struct {
	BaseImpl
}

func NewVNC(hostId string) *VNC {
	return &VNC{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
	}
}

func (vnc *VNC) Code() int32 {
	return types.APP_TYPE_VNC
}

func (vnc *VNC) Preper() error {
	return nil
}

func (vnc *VNC) Dial() error {
	return nil
}

func (vnc *VNC) Response() error {
	cm := conf.NewConfManager("")
	localAddr := fmt.Sprintf("ws://%s:%d", cm.Conf.VNCConf.Websockify.Host, cm.Conf.VNCConf.Websockify.Port)
	logrus.Debug("VNCResponser response ", localAddr)
	vncConn, _, err := websocket.DefaultDialer.Dial(localAddr, nil)
	if err != nil {
		return err
	}
	vnc.PipeServer = vncConn.UnderlyingConn()
	return nil
}
