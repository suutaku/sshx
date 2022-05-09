package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"

	"github.com/suutaku/go-sshfs/pkg/sshfs"

	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
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
	sshfs          *sshfs.Sshfs
}

func NewSfsImpl() *SfsImpl {
	return &SfsImpl{}
}

func (dal *SfsImpl) passwordCallback() (string, error) {
	logrus.Debug("password callback")
	dal.retry++
	if dal.retry > NumberOfPrompts {
		return "", fmt.Errorf("auth failed")
	}
	fmt.Print("Password: ")
	b, _ := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")
	dal.config.Auth = append(dal.config.Auth, ssh.Password(string(b)))
	return string(b), nil
}

func (dal *SfsImpl) Init(param ImplParam) {
	dal.config = ssh.ClientConfig{
		HostKeyCallback: ssh.HostKeyCallback(hostKeyCallback),
		Timeout:         10 * time.Second,
	}
	dal.conn = param.Conn
	dal.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	dal.localSSHAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalSSHPort)
	dal.hostId = param.HostId
	dal.pairId = param.PairId
}

func (dal *SfsImpl) SetPairId(id string) {
	if dal.pairId == "" {
		dal.pairId = id
	}
}

func (dal *SfsImpl) SetRoot(root string) {
	dal.root = root
}

func (dal *SfsImpl) SetMountPoint(mtp string) {
	dal.mountPoint = mtp
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
		dal.Close()
		return err
	}
	return nil
}

func (dal *SfsImpl) DecodeAddress(addrStr string) error {
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

func (dal *SfsImpl) PrivateKeyOption(keyPath string) {
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

func (dal *SfsImpl) Response() error {
	logrus.Debug("Dail local addr ", dal.localSSHAddr)
	conn, err := net.Dial("tcp", dal.localSSHAddr)
	if err != nil {
		return err
	}
	logrus.Debug("Dail local addr ", dal.localSSHAddr, " success")
	dal.conn = &conn
	return nil
}

func (dal *SfsImpl) Close() {

	if dal.sshfs != nil {
		dal.sshfs.Unmount()
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
	// logrus.Debug("create sshfs conn from dal.conn")
	dal.config.Auth = append(dal.config.Auth, ssh.RetryableAuthMethod(ssh.PasswordCallback(dal.passwordCallback), NumberOfPrompts))
	c, chans, reqs, err := ssh.NewClientConn(*dal.conn, "", &dal.config)
	if err != nil {
		return err
	}
	sshClient := ssh.NewClient(c, chans, reqs)
	if sshClient == nil {
		return fmt.Errorf("cannot create ssh client")
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	dal.sshfs = sshfs.NewSshfs(sftpClient, dal.root, dal.mountPoint, dal.hostId)
	if dal.sshfs == nil {
		return fmt.Errorf("cannot create sshfs")
	}
	opts := &fs.Options{}
	if utils.DebugOn() {
		opts.Debug = true
	}
	err = dal.sshfs.Mount(opts)
	if err != nil {
		logrus.Error(err)
	}
	dal.sshfs.Unmount()
	dal.Close()
	logrus.Info("close sfs impl")

	return err
}
