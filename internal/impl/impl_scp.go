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

	"github.com/povsister/scp"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

type ScpImpl struct {
	conn           *net.Conn
	config         ssh.ClientConfig
	localEntryAddr string
	localSSHAddr   string
	hostId         string
	pairId         string
	retry          int
	toRemote       bool
	localPath      string
	remotePath     string
}

func NewScpImpl() *ScpImpl {
	return &ScpImpl{}
}

func (dal *ScpImpl) SetPairId(id string) {
	if dal.pairId == "" {
		dal.pairId = id
	}
}

func (dal *ScpImpl) Init(param ImplParam) {
	dal.config = ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	dal.conn = param.Conn
	dal.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
	dal.localSSHAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalSSHPort)
	dal.hostId = param.HostId
	dal.pairId = param.PairId
}

func (dal *ScpImpl) Code() int32 {
	return types.APP_TYPE_SCP
}

func (dal *ScpImpl) Dial() error {
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
	err = dal.dialAndCopyFiles()
	if err != nil {
		err = dal.RequestPassword(err)
		if err == nil {
			return dal.Dial()
		}
		return err
	}
	return nil
}

func (s *ScpImpl) RequestPassword(err error) error {
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

func (dal *ScpImpl) Response() error {
	logrus.Debug("Dail local addr ", dal.localSSHAddr)
	conn, err := net.Dial("tcp", dal.localSSHAddr)
	if err != nil {
		return err
	}
	logrus.Debug("Dail local addr ", dal.localSSHAddr, " success")
	dal.conn = &conn
	return nil
}

func (dal *ScpImpl) Close() {
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

func (dal *ScpImpl) DialerReader() io.Reader {
	return *dal.conn
}

func (dal *ScpImpl) DialerWriter() io.Writer {
	return *dal.conn
}

func (dal *ScpImpl) ResponserReader() io.Reader {
	return *dal.conn
}

func (dal *ScpImpl) ResponserWriter() io.Writer {
	return *dal.conn
}

func (dal *ScpImpl) dialAndCopyFiles() error {
	logrus.Debug("create scp conn from dal.conn")
	c, chans, reqs, err := ssh.NewClientConn(*dal.conn, "", &dal.config)
	if err != nil {
		return err
	}
	logrus.Debug("conn ok")
	client := ssh.NewClient(c, chans, reqs)
	if client == nil {
		return fmt.Errorf("cannot create ssh client")
	}
	scpClient, err := scp.NewClientFromExistingSSH(client, &scp.ClientOption{})
	if err != nil {
		return nil
	}
	if dal.toRemote {
		isDir, err := isDirectory(dal.localPath)
		if err != nil {
			return err
		}
		if isDir {
			return scpClient.CopyDirToRemote(dal.localPath, dal.remotePath, &scp.DirTransferOption{})
		} else {
			return scpClient.CopyFileToRemote(dal.localPath, dal.remotePath, &scp.FileTransferOption{})
		}

	} else {
		isDir, err := isDirectory(dal.localPath)
		if err != nil {
			return err
		}
		if isDir {
			return scpClient.CopyDirFromRemote(dal.remotePath, dal.localPath, &scp.DirTransferOption{})
		} else {
			return scpClient.CopyFileFromRemote(dal.remotePath, dal.localPath, &scp.FileTransferOption{})
		}

	}
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

func (dal *ScpImpl) PrivateKeyOption(keyPath string) {
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

func (dal *ScpImpl) DecodeAddress(addrStr string) error {
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

func (dal *ScpImpl) ParsePaths(src, dest string) error {
	srcHost, srcPath := dal.splitAddr(src)
	destHost, destPath := dal.splitAddr(dest)
	if srcHost == "" && destHost != "" {
		dal.toRemote = true
		dal.DecodeAddress(destHost)
		if srcPath == "" {
			return fmt.Errorf("empty src path")
		}
		dal.localPath = srcPath
		if destPath == "" {
			return fmt.Errorf("empty src path")
		}
		dal.remotePath = destPath

	} else if srcHost != "" && destHost == "" {
		dal.toRemote = false
		dal.DecodeAddress(srcHost)
		if srcPath == "" {
			return fmt.Errorf("empty src path")
		}
		dal.remotePath = srcPath
		if destPath == "" {
			return fmt.Errorf("empty src path")
		}
		dal.localPath = destPath
	} else {
		return fmt.Errorf("bad param: src host %s, dest host %s", srcHost, destHost)
	}
	return nil
}

func (dal *ScpImpl) splitAddr(addr string) (host, path string) {
	sps := strings.Split(addr, ":")
	if len(sps) < 2 {
		path = sps[0]
	} else {
		host = sps[0]
		path = sps[1]
	}
	return
}
