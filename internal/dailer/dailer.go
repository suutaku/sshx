package dailer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

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
	conf    conf.Configure
}

func NewDailer(conf conf.Configure) *Dailer {
	return &Dailer{
		conf: conf,
	}
}

type x11request struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

type x11channel struct {
	Host string
	Port uint32
}

func (dal *Dailer) X11Request() {
	// x11-req Payload
	payload := x11request{
		SingleConnection: false,
		AuthProtocol:     string("MIT-MAGIC-COOKIE-1"),
		AuthCookie:       string("d92c30482cc3d2de61888961deb74c08"),
		ScreenNumber:     uint32(0),
	}

	// NOTE:
	// send x11-req Request
	ok, err := dal.session.SendRequest("x11-req", true, ssh.Marshal(payload))
	if err == nil && !ok {
		fmt.Println(errors.New("ssh: x11-req failed"))
	}

	// x11 OpenChannel (Not working...)
	x11Data := x11channel{
		Host: "localhost",
		Port: uint32(6000),
	}

	dal.client.OpenChannel("x11", ssh.Marshal(x11Data))
}

func (dal *Dailer) Connect(host, port string, x11 bool, conf ssh.ClientConfig) error {

	var err error
	if conf.Auth == nil || len(conf.Auth) == 0 {
		fmt.Print("Password: ")
		b, _ := terminal.ReadPassword(int(syscall.Stdin))
		// fmt.Scanf("%s\n", &pass)
		fmt.Print("\n")
		conf.Auth = append(conf.Auth, ssh.Password(string(b)))
	}

	switch tools.AddrType(host) {
	case tools.ADDR_TYPE_ID:
		// tcp dail, send key
		conn, err := net.DialTimeout("tcp", dal.conf.LocalListenAddr, time.Second)
		if err != nil {
			return err
		}
		req := proto.ConnectRequest{
			Host: host,
		}
		b, _ := req.Marshal()
		conn.Write(b)
		conn.Read(b)
		c, chans, reqs, err := ssh.NewClientConn(conn, "", &conf)
		if err != nil {
			return err
		}
		dal.client = ssh.NewClient(c, chans, reqs)
	default:
		dal.client, err = ssh.Dial("tcp", host+":"+port, &conf)
		if err != nil {
			return err
		}
	}
	if dal.client == nil {
		return fmt.Errorf("connection faild")
	}
	dal.session, err = dal.client.NewSession()
	if err != nil {
		dal.client.Close()
		return err
	}
	if x11 {
		dal.X11Request()
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

func (dal *Dailer) Close() {
	terminal.Restore(dal.fd, dal.state)
}
