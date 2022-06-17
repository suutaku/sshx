package impl

import (
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/povsister/scp"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/conf"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/crypto/ssh/terminal"
)

const NumberOfPrompts = 3

var keyErr *knownhosts.KeyError

type SSH struct {
	BaseImpl
	X11       bool
	Address   string
	CopyIdOpt bool
	Identify  string
	config    ssh.ClientConfig
}

func NewSSH(address string, x11 bool, ident string, copyId bool) *SSH {
	return &SSH{
		X11:       x11,
		Address:   address,
		CopyIdOpt: copyId,
		Identify:  ident,
	}
}

func (s *SSH) Code() int32 {
	return types.APP_TYPE_SSH
}

func (s *SSH) Preper() error {
	s.config = ssh.ClientConfig{
		HostKeyCallback: ssh.HostKeyCallback(hostKeyCallback),
		Timeout:         timeout,
	}
	s.privateKeyOption()
	err := s.decodeAddress()
	return err
}

func (s *SSH) Dial() error {
	return nil
}

func (s *SSH) Response() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	cm := conf.NewConfManager("")

	logrus.Debug("Dail local addr ", cm.Conf.LocalSSHPort)
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", cm.Conf.LocalSSHPort))
	if err != nil {
		return err
	}
	s.BaseImpl.conn = &conn
	return nil
}

func (s *SSH) privateKeyOption() {
	if s.Identify == "" {
		s.Identify = path.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	}
	pemBytes, err := ioutil.ReadFile(s.Identify)
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
	s.config.Auth = append(s.config.Auth, ssh.PublicKeys(signer))
}

func (s *SSH) decodeAddress() error {
	var userName, addr string
	sps := strings.Split(s.Address, "@")
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
	s.config.User = userName
	s.HId = addr
	return nil
}

// dial remote sshd with opened wrtc connection
func (s *SSH) OpenTerminal(conn net.Conn) error {
	logrus.Debug("dialRemoteAndOpenTerminal")
	s.config.Auth = append(s.config.Auth, ssh.RetryableAuthMethod(ssh.PasswordCallback(s.passwordCallback), NumberOfPrompts))
	c, chans, reqs, err := ssh.NewClientConn(conn, "", &s.config)
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
	if s.CopyIdOpt {
		scpClient, err := scp.NewClientFromExistingSSH(client, &scp.ClientOption{})
		if err != nil {
			return nil
		}
		pubKeyPath := path.Join(os.Getenv("HOME"), ".ssh", "id_rsa.pub")
		tmplatePath := path.Join("tmp", "")
		targetKey := path.Join("~", "./.ssh/authorized_keys")
		err = scpClient.CopyFileToRemote(pubKeyPath, tmplatePath, &scp.FileTransferOption{Perm: os.FileMode(0600)})
		if err != nil {
			logrus.Warn(err)
		} else {
			session.Run("cat " + tmplatePath + " >> " + targetKey)
			session.Run("rm " + tmplatePath)
		}

		return nil

	}
	if s.X11 {
		logrus.Debug("x11 enable")
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
	logrus.Debug("wait session")
	return session.Wait()

}

func (dal *SSH) passwordCallback() (string, error) {
	logrus.Debug("password callback")
	fmt.Print("Password: ")
	b, _ := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	dal.config.Auth = append(dal.config.Auth, ssh.Password(string(b)))
	return string(b), nil
}

func createKnownHosts() {
	f, fErr := os.OpenFile(path.Join(os.Getenv("HOME"), ".ssh", "known_hosts"), os.O_CREATE, 0600)
	if fErr != nil {
		logrus.Error(fErr)
	}
	f.Close()
}

func checkKnownHosts() ssh.HostKeyCallback {
	createKnownHosts()
	kh, err := knownhosts.New(path.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		logrus.Error(err)
	}
	return kh
}

func hostKeyString(k ssh.PublicKey) string {
	return k.Type() + " " + base64.StdEncoding.EncodeToString(k.Marshal()) // e.g. "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY...."
}

func hostKeyCallback(host string, remote net.Addr, pubKey ssh.PublicKey) error {
	kh := checkKnownHosts()
	if kh == nil {
		return nil
	}
	hErr := kh(host, remote, pubKey)
	// Reference: https://blog.golang.org/go1.13-errors
	// To understand what errors.As is.
	if hErr == nil {
		//logrus.Printf("Pub key exists for %s.", host)
		return nil
	}
	if errors.As(hErr, &keyErr) && len(keyErr.Want) > 0 {
		// Reference: https://www.godoc.org/golang.org/x/crypto/ssh/knownhosts#KeyError
		// if keyErr.Want slice is empty then host is unknown, if keyErr.Want is not empty
		// and if host is known then there is key mismatch the connection is then rejected.
		//	logrus.Printf("WARNING: %v is not a key of %s, either a MiTM attack or %s has reconfigured the host pub key.", hostKeyString(pubKey), host, host)
		// return keyErr
		// force continue (NOT SAFE!!!)
		return nil
	} else if errors.As(hErr, &keyErr) && len(keyErr.Want) == 0 {
		// host key not found in known_hosts then give a warning and continue to connect.
		//logrus.Printf("WARNING: %s is not trusted, adding this key: %q to known_hosts file.", host, hostKeyString(pubKey))
		return addHostKey(host, remote, pubKey)
	}
	//logrus.Printf("Pub key exists for %s.", host)
	return nil
}

func addHostKey(host string, remote net.Addr, pubKey ssh.PublicKey) error {
	// add host key if host is not found in known_hosts, error object is return, if nil then connection proceeds,
	// if not nil then connection stops.
	khFilePath := path.Join(os.Getenv("HOME"), ".ssh", "known_hosts")

	f, fErr := os.OpenFile(khFilePath, os.O_APPEND|os.O_WRONLY, 0600)
	if fErr != nil {
		return fErr
	}
	defer f.Close()

	knownHosts := knownhosts.Normalize(remote.String())
	_, fileErr := f.WriteString(knownhosts.Line([]string{knownHosts}, pubKey) + "\n")
	return fileErr
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
	logrus.Debug("x11Request")
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
	logrus.Debug("create X11 socket")
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
