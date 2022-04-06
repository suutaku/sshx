package dailer

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/conf"
	"github.com/suutaku/sshx/internal/proto"
	"github.com/suutaku/sshx/internal/tools"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type Dailer struct {
	ctx     context.Context
	client  *ssh.Client
	session *ssh.Session
	state   *terminal.State
	fd      int
	Conf    conf.Configure
}

func NewDailer(conf conf.Configure) *Dailer {
	return &Dailer{
		Conf: conf,
		ctx:  context.Background(),
	}
}
func (dal *Dailer) RequstPassword(conf *ssh.ClientConfig) {
	fmt.Print("Password: ")
	b, _ := terminal.ReadPassword(int(syscall.Stdin))
	// fmt.Scanf("%s\n", &pass)
	fmt.Print("\n")
	conf.Auth = append(conf.Auth, ssh.Password(string(b)))
}

func (dal *Dailer) OpenTerminal(req proto.ConnectRequest, conf ssh.ClientConfig) error {
	err := dal.Connect(req, conf)
	if err != nil {
		logrus.Error("after call connect", err)
		return err
	}
	dal.fd = int(os.Stdin.Fd())

	dal.state, err = terminal.MakeRaw(dal.fd)
	if err != nil {
		return fmt.Errorf("terminal make raw: %s", err)
	}
	w, h, err := terminal.GetSize(dal.fd)
	if err != nil {
		return fmt.Errorf("terminal get size: %s", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}
	if err := dal.session.RequestPty(term, h, w, modes); err != nil {
		return fmt.Errorf("session xterm: %s", err)
	}

	dal.session.Stdout = os.Stdout
	dal.session.Stderr = os.Stderr
	dal.session.Stdin = os.Stdin
	if err := dal.session.Shell(); err != nil {
		return fmt.Errorf("session shell: %s", err)
	}

	if err := dal.session.Wait(); err != nil {
		if e, ok := err.(*ssh.ExitError); ok {
			switch e.ExitStatus() {
			case 130:
				return nil
			}
		}
		return fmt.Errorf("ssh: %s", err)
	}
	return nil
}

func (dal *Dailer) Connect(req proto.ConnectRequest, config ssh.ClientConfig) error {

	var err error
	if config.Auth == nil || len(config.Auth) == 0 {
		dal.RequstPassword(&config)
	}

	switch tools.AddrType(req.Host) {
	case tools.ADDR_TYPE_ID:
		// tcp dail, send key
		conn, err := net.DialTimeout("tcp", dal.Conf.LocalListenAddr, time.Second)
		if err != nil {
			return err
		}
		b, _ := req.Marshal()
		conn.Write(b)
		conn.Read(b)
		if req.Type == conf.TYPE_START_PROXY {
			break
		}
		c, chans, reqs, err := ssh.NewClientConn(conn, "", &config)
		if err != nil {
			config.Auth = make([]ssh.AuthMethod, 0)
			req.Timestamp = time.Now().Unix()
			return dal.Connect(req, config)
		}
		dal.client = ssh.NewClient(c, chans, reqs)
	default:
		dal.client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", req.Host, req.Port), &config)
		if err != nil {
			if strings.Contains(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
				config.Auth = make([]ssh.AuthMethod, 0)
				req.Timestamp = time.Now().Unix()
				return dal.Connect(req, config)
			} else {
				return err
			}
		}
	}
	if req.Type != conf.TYPE_START_PROXY {
		if dal.client == nil {
			return fmt.Errorf("connection faild")
		}
		dal.session, err = dal.client.NewSession()
		if err != nil {
			dal.client.Close()
			return err
		}
	}
	if req.X11 {
		dal.X11Request()
	}
	return nil
}

func (dal *Dailer) Close(req proto.ConnectRequest) {
	if dal.state != nil {
		terminal.Restore(dal.fd, dal.state)
	}
	if dal.session != nil {
		dal.session.Close()
	}
	if dal.client != nil {
		dal.client.Close()
	}
	conn, err := net.DialTimeout("tcp", dal.Conf.LocalListenAddr, time.Second)
	if err != nil {
		logrus.Error(err)
	}
	req.Type = conf.TYPE_CLOSE_CONNECTION
	b, _ := req.Marshal()
	conn.Write(b)
	conn.Read(b)
	conn.Close()
}

func (dal *Dailer) CloseProxy(id string) {
	conn, err := net.DialTimeout("tcp", dal.Conf.LocalListenAddr, time.Second)
	if err != nil {
		logrus.Error(err)
	}
	req := proto.ConnectRequest{
		Type: conf.TYPE_STOP_PROXY,
		Host: id,
	}
	b, _ := req.Marshal()
	conn.Write(b)
	conn.Read(b)
}

func (dal *Dailer) GetProxyList(host string) (ret proto.ListDestroyResponse) {
	conn, err := net.DialTimeout("tcp", dal.Conf.LocalListenAddr, time.Second)
	if err != nil {
		logrus.Error(err)
		return ret
	}
	defer conn.Close()
	req := proto.ConnectRequest{
		Type: conf.TYPE_PROXY_LIST,
		Host: host,
	}
	buf := make([]byte, 1024)
	b, _ := req.Marshal()
	conn.Write(b)
	conn.Read(b)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.Read(buf)
	ret.Unmarshal(buf)
	return ret
}
