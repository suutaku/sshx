package dailer

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

func (dal *Dailer) Transfer(filePath, remotePath string, upload bool, scpClient *scp.Client, mode fs.FileMode) error {

	var file *os.File
	var err error

	if upload {
		log.Println("start upload: ", filePath, " to ", scpClient.Host, remotePath)
		file, err = os.Open(filePath)
	} else {
		log.Println("start download: ", filePath, " to ", scpClient.Host, remotePath)
		file, err = os.Create(filePath)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if upload {
		return scpClient.CopyFromFile(context.Background(), *file, filepath.Join(remotePath, path.Base(file.Name())), fmt.Sprintf("%04o", mode.Perm()))
	}
	return scpClient.CopyFromRemote(context.Background(), file, filepath.Join(remotePath, path.Base(file.Name())))
}

func (dal *Dailer) Copy(localPath, remotePath, host, port string, upload bool, conf ssh.ClientConfig) error {
	log.Println("localPath", localPath)
	log.Println("remotePath", remotePath)
	log.Println("host", host)
	log.Println("port", port)
	err := dal.Connect(host, port, false, conf)
	if err != nil {
		return err
	}
	scpClient, err := scp.NewClientBySSH(dal.client)
	if err != nil {
		return err
	}
	err = scpClient.Connect()
	if err != nil {
		return err
	}
	defer scpClient.Close()

	if upload {
		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer file.Close()
		fileInfo, err := file.Stat()
		if err != nil {
			return err
		}
		if fileInfo.IsDir() {
			files, err := ioutil.ReadDir(localPath)
			if err != nil {
				return err
			}
			for _, f := range files {
				fmt.Println(f.Name())
				return dal.Transfer(f.Name(), remotePath, upload, &scpClient, f.Mode())
			}
		} else {
			return dal.Transfer(localPath, remotePath, upload, &scpClient, fileInfo.Mode())
		}
	} else {
		return dal.Transfer(localPath, remotePath, upload, &scpClient, 0)
	}

	return nil
}