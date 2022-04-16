package impl

import (
	"crypto/x509"
	"encoding/gob"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const maxRetryTime = 3

type SshImpl struct {
	conn           *net.Conn
	config         ssh.ClientConfig
	localEntryAddr string
	localSSHAddr   string
	x11            bool
	hostId         string
	pairId         string
	retry          int
}

func NewSshImpl() *SshImpl {
	return &SshImpl{}
}

func (vnc *SshImpl) Init(param ImplParam) {
	vnc.config = ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	vnc.conn = param.Conn
	vnc.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	vnc.localSSHAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalSSHPort)
	vnc.hostId = param.HostId
	vnc.pairId = param.PairId
}

func (dal *SshImpl) Conn() *net.Conn {
	return dal.conn
}

func (dal *SshImpl) Code() int32 {
	return types.APP_TYPE_SSH
}

func (dal *SshImpl) SetPairId(id string) {
	if dal.pairId == "" {
		dal.pairId = id
	}
}

func (dal *SshImpl) Close() {
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
	err = enc.Encode(req)
	if err != nil {
		return
	}
	logrus.Debug("close ssh impl")
}

func (s *SshImpl) Response() error {
	logrus.Debug("Dail local addr ", s.localSSHAddr)
	conn, err := net.Dial("tcp", s.localSSHAddr)
	if err != nil {
		return err
	}
	logrus.Debug("Dail local addr ", s.localSSHAddr, " success")
	s.conn = &conn
	return nil
}

func (dal *SshImpl) Dial() error {
	conn, err := net.Dial("tcp", dal.localEntryAddr)
	if err != nil {
		return err
	}
	req := NewCoreRequest(dal.Code(), types.OPTION_TYPE_UP)
	req.PairId = []byte(dal.pairId)
	req.Payload = []byte(dal.hostId)

	if err := gob.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	logrus.Debug("waitting TCP Response")

	resp := CoreResponse{}
	if err := gob.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	logrus.Debug("TCP Response comming")
	if resp.Status != 0 {
		return err
	}
	dal.pairId = string(resp.PairId)
	dal.conn = &conn
	err = dal.dialRemoteAndOpenTerminal()
	if err != nil {
		err = dal.RequestPassword(err)
		if err == nil {
			dal.Close()
			return dal.Dial()
		}
		return err
	}
	return nil
}

func (s *SshImpl) RequestPassword(err error) error {
	if s.retry >= maxRetryTime {
		return fmt.Errorf("invalid password")
	}
	if strings.Contains(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
		s.config.Auth = make([]ssh.AuthMethod, 0)
		s.retry++
		logrus.Debug("retry at ", s.retry)
		fmt.Print("Password: ")
		b, _ := terminal.ReadPassword(int(syscall.Stdin))
		fmt.Print("\n")
		s.config.Auth = append(s.config.Auth, ssh.Password(string(b)))
		return nil
	}
	return err
}

// dial remote sshd with opened wrtc connection
func (s *SshImpl) dialRemoteAndOpenTerminal() error {
	c, chans, reqs, err := ssh.NewClientConn(*s.conn, "", &s.config)
	if err != nil {
		return err
	}
	logrus.Debug("conn ok")
	client := ssh.NewClient(c, chans, reqs)
	if client == nil {
		return fmt.Errorf("cannot create ssh client")
	}
	logrus.Debug("client ok")
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	logrus.Debug("session")
	if s.x11 {
		x11Request(session, client)
	}
	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		return err
	}
	w, h, err := terminal.GetSize(fd)
	if err != nil {
		return err
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
	if err := session.RequestPty(term, h, w, modes); err != nil {
		return err
	}
	logrus.Debug("pty ok")
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	if err := session.Shell(); err != nil {
		return err
	}
	logrus.Debug("shell ok")
	defer session.Close()
	defer client.Close()
	defer terminal.Restore(fd, state)

	if err := session.Wait(); err != nil {
		if e, ok := err.(*ssh.ExitError); ok {
			switch e.ExitStatus() {
			case 130:
				return nil
			}
		}
		return err
	}
	return nil

}

func (dal *SshImpl) PrivateKeyOption(keyPath string) {
	if keyPath == "" {
		home := os.Getenv("HOME")
		keyPath = home + "/.ssh/id_rsa"
	}
	pemBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		logrus.Printf("Reading private key file failed %v", err)
		return
	}
	// create signer
	signer, err := SignerFromPem(pemBytes, nil)
	if err != nil {
		logrus.Error(err)
		return
	}
	dal.config.Auth = append(dal.config.Auth, ssh.PublicKeys(signer))
}

func (dal *SshImpl) X11Option(enable bool) {
	dal.x11 = enable
}

const (
	ADDR_TYPE_IPV4 = iota
	ADDR_TYPE_IPV6
	ADDR_TYPE_DOMAIN
	ADDR_TYPE_ID
)

func AddrType(addrStr string) int {
	addr := net.ParseIP(addrStr)
	if addr != nil {
		return ADDR_TYPE_IPV4
	}
	if strings.Contains(addrStr, ".") {
		return ADDR_TYPE_DOMAIN
	}

	return ADDR_TYPE_ID

}

func (dal *SshImpl) DecodeAddress(addrStr string) error {
	var userName, addr string
	sps := strings.Split(addrStr, "@")
	if len(sps) < 2 {
		user, err := user.Current()
		if err != nil {
			return err
		}
		userName = user.Username
		addr = sps[0]
	} else {
		userName = sps[0]
		addr = sps[1]
	}
	dal.config.User = userName
	dal.hostId = addr
	return nil
}

func SignerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {

	// read pem block
	err := errors.New("pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing plain private key failed %v", err)
	}

	return signer, nil
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("parsing private key failed, unsupported key type %q", block.Type)
	}
}

/*
	X11 tools
*/
type x11request struct {
	SingleConnection bool
	AuthProtocol     string
	AuthCookie       string
	ScreenNumber     uint32
}

func x11Request(session *ssh.Session, client *ssh.Client) {
	// x11-req Payload
	payload := x11request{
		SingleConnection: false,
		AuthProtocol:     string("MIT-MAGIC-COOKIE-1"),
		AuthCookie:       string("d92c30482cc3d2de61888961deb74c08"),
		ScreenNumber:     uint32(0),
	}

	// NOTE:
	// send x11-req Request
	ok, err := session.SendRequest("x11-req", true, ssh.Marshal(payload))
	if err == nil && !ok {
		logrus.Error("ssh: x11-req failed")
		return
	}
	x11channels := client.HandleChannelOpen("x11")

	go func() {
		for ch := range x11channels {
			channel, _, err := ch.Accept()
			if err != nil {
				continue
			}

			go forwardX11Socket(channel)
		}
	}()
}

func forwardX11Socket(channel ssh.Channel) {
	conn, err := net.Dial("unix", os.Getenv("DISPLAY"))
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(conn, channel)
		conn.(*net.UnixConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(channel, conn)
		channel.CloseWrite()
		wg.Done()
	}()

	wg.Wait()
	conn.Close()
	channel.Close()
}
