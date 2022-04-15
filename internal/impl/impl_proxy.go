package impl

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
)

type ProxyImpl struct {
	conn           *net.Conn
	localEntryAddr string
	localSSHAddr   string
	hostId         string
	pairId         string
	retry          int
	proxyPort      int32
	running        bool
}

func NewProxyImpl() *ProxyImpl {
	return &ProxyImpl{}
}

func (proxy *ProxyImpl) Init(param ImplParam) {
	proxy.conn = param.Conn
	proxy.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	proxy.localSSHAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalSSHPort)
	proxy.hostId = param.HostId
	proxy.pairId = param.PairId
}

func (proxy *ProxyImpl) Code() int32 {
	return types.APP_TYPE_PROXY
}

func (proxy *ProxyImpl) SetPairId(id string) {
	if proxy.pairId == "" {
		proxy.pairId = id
	}
}

func (proxy *ProxyImpl) doDial(inconn *net.Conn) error {
	conn, err := net.Dial("tcp", proxy.localEntryAddr)
	if err != nil {
		return err
	}

	req := NewCoreRequest(proxy.Code(), types.OPTION_TYPE_UP)
	req.PairId = []byte(proxy.pairId)
	req.Payload = []byte(proxy.hostId)

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
	proxy.pairId = string(resp.PairId)
	utils.Pipe(*inconn, conn)
	return nil
}

func (proxy *ProxyImpl) SetProxyPort(port int32) {
	proxy.proxyPort = port
}

func (proxy *ProxyImpl) Dial() error {
	proxy.running = true
	listenner, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", proxy.proxyPort))
	if err != nil {
		return err
	}
	fmt.Println("Proxy for ", proxy.hostId, " at :", proxy.proxyPort)
	for proxy.running {
		conn, err := listenner.Accept()
		if err != nil {
			continue
		}
		// proxy.conn = &conn
		go proxy.doDial(&conn)

	}
	logrus.Debug("Close proxy for ", proxy.hostId)
	return nil
}

func (proxy *ProxyImpl) Response() error {
	logrus.Debug("Dail local addr ", proxy.localSSHAddr)
	conn, err := net.Dial("tcp", proxy.localSSHAddr)
	if err != nil {
		return err
	}
	logrus.Debug("Dail local addr ", proxy.localSSHAddr, " success")
	proxy.conn = &conn
	return nil
}

func (proxy *ProxyImpl) Close() {
	if proxy.conn != nil {
		(*proxy.conn).Close()
	}
	proxy.running = false
}

func (proxy *ProxyImpl) Conn() *net.Conn {
	return proxy.conn
}
