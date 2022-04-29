package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-sshfs/pkg/fs"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type SfsImpl struct {
	conn           *net.Conn
	localEntryAddr string
	localSSHAddr   string
	hostId         string
	pairId         string
	config         ssh.ClientConfig
	retry          int
	root           string
	mountPoint     string
	sshfs          *fs.SSHFS
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
	dal.conn = &conn
	err = dal.dialAndMount()
	if err != nil {
		err = dal.RequestPassword(err)
		if err == nil {
			return dal.Dial()
		}
		return err
	}
	return nil
}

func (s *SfsImpl) RequestPassword(err error) error {
	if s.retry >= maxRetryTime {
		return fmt.Errorf("invalid password")
	}
	if strings.Contains(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
		s.config.Auth = make([]ssh.AuthMethod, 0)
		s.retry++
		logrus.Debug("retry at ", s.retry)
		fmt.Print("Password: ")
		b, _ := term.ReadPassword(int(syscall.Stdin))
		fmt.Print("\n")
		s.config.Auth = append(s.config.Auth, ssh.Password(string(b)))
		return nil
	}
	return err
}

func (dal *SfsImpl) Response() error {
	return nil
}

func (dal *SfsImpl) Close() {

	if dal.sshfs != nil {
		dal.sshfs.Close()
	}
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
	if dal.conn != nil {
		(*dal.conn).Close()
	}
}

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

func (dal *SfsImpl) dialAndMount() error {
	logrus.Debug("create scp conn from dal.conn")

	logrus.Debug("conn ok")

	dal.sshfs = fs.NewSSHFSWithConn(*dal.conn, dal.config, dal.root, dal.mountPoint)
	err := dal.sshfs.Mount()
	if err != nil {
		dal.sshfs.Unmount()
	}
	return nil
}
