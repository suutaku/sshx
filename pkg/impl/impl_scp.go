package impl

import (
	"fmt"
	"os"
	"strings"

	"github.com/povsister/scp"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
)

type SCP struct {
	BaseImpl
	ToRemote      bool
	LocalPath     string
	RemotePath    string
	Identiry      string
	TargetAddress string
}

func NewSCP(src, dest, ident string) *SCP {
	ret := &SCP{
		Identiry: ident,
	}
	err := ret.ParsePaths(src, dest)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	return ret
}

func (s *SCP) Preper() error {
	return nil
}

func (s *SCP) Code() int32 {
	return types.APP_TYPE_SCP
}

func (s *SCP) Dial() error {
	ssht := NewSSH(s.TargetAddress, false, s.Identiry, false)
	err := ssht.Preper()
	if err != nil {
		logrus.Error(err)
		return err
	}

	sender := NewSender(ssht, types.OPTION_TYPE_UP)
	conn, err := sender.Send()
	if err != nil {
		logrus.Error(err)
		return err
	}

	logrus.Debug("create scp conn from dal.conn")
	ssht.config.Auth = append(ssht.config.Auth, ssh.RetryableAuthMethod(ssh.PasswordCallback(ssht.passwordCallback), NumberOfPrompts))
	c, chans, reqs, err := ssh.NewClientConn(conn, "", &ssht.config)
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
	if s.ToRemote {
		isDir, err := isDirectory(s.LocalPath)
		if err != nil {
			return err
		}
		if isDir {
			return scpClient.CopyDirToRemote(s.LocalPath, s.RemotePath, &scp.DirTransferOption{})
		} else {
			return scpClient.CopyFileToRemote(s.LocalPath, s.RemotePath, &scp.FileTransferOption{})
		}

	} else {
		isDir, err := isDirectory(s.LocalPath)
		if err != nil {
			return err
		}
		if isDir {
			return scpClient.CopyDirFromRemote(s.RemotePath, s.LocalPath, &scp.DirTransferOption{})
		} else {
			return scpClient.CopyFileFromRemote(s.RemotePath, s.LocalPath, &scp.FileTransferOption{})
		}

	}
}

func (s *SCP) Response() error {
	return nil
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

func (s *SCP) ParsePaths(src, dest string) error {
	srcHost, srcPath := s.splitAddr(src)
	destHost, destPath := s.splitAddr(dest)
	if srcHost == "" && destHost != "" {
		s.ToRemote = true
		s.LocalPath = srcPath
		s.RemotePath = destPath
		s.TargetAddress = destHost

	} else if srcHost != "" && destHost == "" {
		s.ToRemote = false
		s.RemotePath = srcPath
		s.LocalPath = destPath
		s.TargetAddress = srcHost
	} else {
		return fmt.Errorf("bad param: src host %s, dest host %s", srcHost, destHost)
	}
	return nil
}

func (s *SCP) splitAddr(addr string) (host, path string) {
	sps := strings.Split(addr, ":")
	if len(sps) < 2 {
		path = sps[0]
	} else {
		host = sps[0]
		path = sps[1]
	}
	return
}
