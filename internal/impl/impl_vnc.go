package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

/*
	we use noVNC as a dialer, so dialer impl is empty
*/

type VNCImpl struct {
	localEntryAddr string
	localVNCAddr   string
	hostId         string
	pairId         string
	wsconn         *websocket.Conn
	tcpconn        *net.Conn
}

func NewVNCImpl() *VNCImpl {
	return &VNCImpl{}
}

func (vnc *VNCImpl) Init(param ImplParam) {
	vnc.tcpconn = param.Conn
	vnc.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	vnc.localVNCAddr = fmt.Sprintf("ws://127.0.0.1:%d", param.Config.VNCConf.Websockify.Port)
	vnc.hostId = param.HostId
	vnc.pairId = param.PairId
}

func (dal *VNCImpl) Code() int32 {
	return types.APP_TYPE_VNC
}

func (dal *VNCImpl) SetPairId(id string) {
	if dal.pairId == "" {
		dal.pairId = id
	}
}

func (dal *VNCImpl) Dial() error {

	conn, err := net.Dial("tcp", dal.localEntryAddr)
	if err != nil {
		return err
	}

	req := NewCoreRequest(dal.Code(), types.OPTION_TYPE_UP)
	req.PairId = []byte(dal.pairId)
	req.Payload = []byte(dal.hostId)

	enc := gob.NewEncoder(conn)
	err = enc.Encode(req)
	if err != nil {
		return err
	}
	logrus.Debug("waitting TCP Response")
	resp := CoreResponse{}
	dec := gob.NewDecoder(conn)
	err = dec.Decode(&resp)
	if err != nil {
		return err
	}
	logrus.Debug("TCP Response comming")

	if resp.Status != 0 {
		return err
	}
	dal.pairId = string(resp.PairId)
	dal.tcpconn = &conn

	return nil
}

type Wraper struct {
	websocket.Conn
}

func (wp Wraper) Read(b []byte) (int, error) {
	return wp.UnderlyingConn().Read(b)
}

func (wp Wraper) Write(b []byte) (int, error) {
	return wp.UnderlyingConn().Write(b)
}

func (dal *VNCImpl) Response() error {
	logrus.Debug("VNCResponser response ", dal.localVNCAddr)
	vnc, _, err := websocket.DefaultDialer.Dial(dal.localVNCAddr, nil)
	if err != nil {
		return err
	}
	dal.wsconn = vnc
	return nil
}

func (dal *VNCImpl) Close() {
	req := NewCoreRequest(dal.Code(), types.OPTION_TYPE_DOWN)
	req.PairId = []byte(dal.pairId)
	req.Payload = []byte(dal.hostId)

	// new request, beacuase originnal ssh connection was closed
	conn, err := net.Dial("tcp", dal.localEntryAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	enc.Encode(req)
	logrus.Info("vnc impl close")
}

func (dal *VNCImpl) DialerWriter() io.Writer {
	for dal.tcpconn == nil {
		logrus.Warn("DialerWriter watting connection")
		time.Sleep(200 * time.Millisecond)
	}
	return *dal.tcpconn
}

func (dal *VNCImpl) DialerReader() io.Reader {
	for dal.tcpconn == nil {
		logrus.Warn("DialerReader watting connection")
		time.Sleep(200 * time.Millisecond)
	}
	return *dal.tcpconn
}

func (dal *VNCImpl) ResponserWriter() io.Writer {
	for dal.wsconn == nil {
		logrus.Warn("ResponserWriter watting connection")
		time.Sleep(200 * time.Millisecond)
	}
	return Wraper{*dal.wsconn}
}

func (dal *VNCImpl) ResponserReader() io.Reader {
	for dal.wsconn == nil {
		logrus.Warn("ResponserReader watting connection")
		time.Sleep(200 * time.Millisecond)
	}
	return Wraper{*dal.wsconn}
}
