package impl

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
)

type SfsImpl struct {
	conn           *net.Conn
	localEntryAddr string
	localSSHAddr   string
	hostId         string
	pairId         string
	config         ssh.ClientConfig
}

func (vnc *SfsImpl) Init(param ImplParam) {
	vnc.config = ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	vnc.conn = param.Conn
	vnc.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	vnc.localSSHAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	vnc.hostId = param.HostId
	vnc.pairId = param.PairId
}

func (dal *SfsImpl) SetPairId(id string) {
	if dal.pairId == "" {
		dal.pairId = id
	}
}

func (dal *SfsImpl) Code() int32 {
	return types.APP_TYPE_SFS
}
func (dal *SfsImpl) Dial() error {
	return nil
}

func (dal *SfsImpl) Response() error {
	return nil
}

func (dal *SfsImpl) Close() {}

func (dal *SfsImpl) DialerReader() io.Reader {
	return *dal.conn
}

func (dal *SfsImpl) DialerWriter() io.Writer {
	return *dal.conn
}

func (dal *SfsImpl) ResponserReader() io.Reader {
	return *dal.conn
}

func (dal *SfsImpl) ResponserWriter() io.Writer {
	return *dal.conn
}
