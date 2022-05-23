package impl

import (
	"fmt"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-sshfs/pkg/sshfs"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
	"golang.org/x/crypto/ssh"
)

type SSHFS struct {
	BaseImpl
	MountPoint string
	Root       string
	Address    string
	sshfs      *sshfs.Sshfs
	Identify   string
}

func NewSSHFS(mountPoint, root, address, id string) *SSHFS {
	return &SSHFS{
		MountPoint: mountPoint,
		Root:       root,
		Address:    address,
		Identify:   id,
	}
}

func (fs *SSHFS) Preper() error {
	// use ssh impl to get host id
	ssht := NewSSH(fs.Address, false, fs.Identify, false)
	err := ssht.Preper()
	if err != nil {
		return err
	}
	fs.HId = ssht.HId
	return nil
}

func (fs *SSHFS) Code() int32 {
	return types.APP_TYPE_SFS
}

func (fs *SSHFS) Dial() error {
	ssht := NewSSH(fs.Address, false, fs.Identify, false)
	err := ssht.Preper()
	if err != nil {
		return err
	}
	fs.HId = ssht.HId
	sender := NewSender(ssht, types.OPTION_TYPE_UP)
	conn, err := sender.Send()
	if err != nil {
		return err
	}
	defer func() {
		conn.Close()
		closeSender := NewSender(ssht, types.OPTION_TYPE_DOWN)
		closeSender.PairId = sender.PairId
		closeSender.SendDetach()
	}()
	ssht.config.Auth = append(ssht.config.Auth, ssh.RetryableAuthMethod(ssh.PasswordCallback(ssht.passwordCallback), NumberOfPrompts))
	c, chans, reqs, err := ssh.NewClientConn(conn, "", &ssht.config)
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
	fs.sshfs = sshfs.NewSshfs(sftpClient, fs.Root, fs.MountPoint, fs.HId)
	if fs.sshfs == nil {
		return fmt.Errorf("cannot create sshfs")
	}
	opts := &fusefs.Options{}
	if utils.DebugOn() {
		opts.Debug = true
	}
	err = fs.sshfs.Mount(opts)
	if err != nil {
		logrus.Error(err)
		fs.Close()
		return err
	}
	logrus.Info("close sfs impl")
	return nil
}

func (fs *SSHFS) Response() error {
	return nil
}

func (fs *SSHFS) Close() {
	fs.BaseImpl.Close()
	fs.sshfs.Unmount()
	logrus.Info("close sfs impl")
}
