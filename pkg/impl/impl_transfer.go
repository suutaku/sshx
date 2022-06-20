package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	TYPE_UPLOAD = iota
	TYPE_DOWNLOAD
)

type FileInfo struct {
	Name       string
	Size       int64
	OptionType int32
	Ready      bool
}

type TransferStatus struct {
	Status int32
}

type Transfer struct {
	BaseImpl
	FilePath string
	Upload   bool
	Size     int64
	FileName string
}

func NewTransfer(hostId string, filePath string, upload bool, header *multipart.FileHeader) *Transfer {
	if filePath == "" && header == nil {
		return nil
	}
	ret := &Transfer{
		BaseImpl: *NewBaseImpl(hostId),
		FilePath: filePath,
		Upload:   upload,
	}
	if header != nil {
		ret.Size = header.Size
		ret.FileName = header.Filename
	}
	return ret
}

func (tr *Transfer) Code() int32 {
	return types.APP_TYPE_TRANSFER
}

func (tr *Transfer) sendHeader() (FileInfo, error) {
	info := FileInfo{
		Name:       tr.FilePath,
		OptionType: TYPE_DOWNLOAD,
	}
	if tr.Upload {
		info.OptionType = TYPE_UPLOAD
		if tr.Size > 0 && tr.FileName != "" {
			info.Size = tr.Size
			info.Name = filepath.Base(tr.FileName)
		} else {
			fInfo, err := os.Stat(tr.FilePath)
			if err != nil {
				logrus.Error(err)
				return info, err
			}
			info.Size = fInfo.Size()
			info.Name = filepath.Base(tr.FilePath)
		}
		info.Ready = true
	}
	err := gob.NewEncoder(tr.Conn()).Encode(info)
	if err != nil {
		return info, err
	}
	err = gob.NewDecoder(tr.Conn()).Decode(&info)
	if err != nil {
		return info, err
	}
	return info, nil
}

func (tr *Transfer) recvHeader(conn net.Conn) (FileInfo, error) {
	info := FileInfo{}
	err := gob.NewDecoder(conn).Decode(&info)
	if err != nil {
		logrus.Error(err)
		return info, err
	}

	if info.OptionType == TYPE_DOWNLOAD {
		tr.FilePath = info.Name
		fInfo, err := os.Stat(tr.FilePath)
		if err != nil {
			logrus.Error(err, tr.FilePath)
			return info, err
		}
		info.Size = fInfo.Size()
		info.Ready = true
	}
	err = gob.NewEncoder(conn).Encode(&info)
	if err != nil {
		logrus.Error(err)
		return info, err
	}
	return info, nil
}
func (tr *Transfer) doResponse(s net.Conn) error {
	// get file header
	info, err := tr.recvHeader(s)
	if err != nil {
		return err
	}
	if !info.Ready {
		return fmt.Errorf("remote not ready")
	}
	switch info.OptionType {
	case TYPE_DOWNLOAD:
		logrus.Debug("response download")
		file, err := os.Open(tr.FilePath)
		if err != nil {
			logrus.Error(err)
			return err
		}
		defer file.Close()
		bar := progressbar.DefaultBytes(
			info.Size,
			"update",
		)
		_, err = io.Copy(io.MultiWriter(s, bar), file)
		s.Close()
		return err
	case TYPE_UPLOAD:
		logrus.Debug("response upload")
		file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", info.Name))
		if err != nil {
			logrus.Error(err)
			return err
		}
		logrus.Debug("file created")
		defer file.Close()
		bar := progressbar.DefaultBytes(
			info.Size,
			"download",
		)
		_, err = io.Copy(io.MultiWriter(file, bar), s)
		return err
	default:
		logrus.Error("invalid file option type for ", info.OptionType)
	}
	// progress
	return nil

}

func (tr *Transfer) Response() error {
	s, c := net.Pipe()
	tr.lock.Lock()
	tr.BaseImpl.conn = &c
	tr.lock.Unlock()
	go func() {
		err := tr.doResponse(s)
		if err != nil {
			logrus.Error("do response ", err)
		}
	}()
	return nil
}

func (tr *Transfer) DoUpload(reader io.Reader) error {
	info, err := tr.sendHeader()
	if err != nil {
		return err
	}
	bar := progressbar.DefaultBytes(
		info.Size,
		"upload",
	)

	if reader == nil {
		file, err := os.Open(tr.FilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		n, err := io.Copy(io.MultiWriter(tr.Conn(), bar), file)
		logrus.Debug("stop process upload ", err, n)
		// time.Sleep(5 * time.Second)
		return err
	} else {
		n, err := io.Copy(io.MultiWriter(tr.Conn(), bar), reader)
		logrus.Warn("use file discriber and do noting", n, err)
	}
	return nil
}

func (tr *Transfer) DoDownload(writer io.Writer) error {
	info, err := tr.sendHeader()
	if err != nil {
		return err
	}
	bar := progressbar.DefaultBytes(
		info.Size,
		"download",
	)
	if writer == nil {
		file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", filepath.Base(info.Name)))
		if err != nil {
			logrus.Error(err)
			return err
		}
		defer file.Close()
		io.Copy(io.MultiWriter(file, bar), tr.Conn())
	} else {
		io.Copy(io.MultiWriter(writer, bar), tr.Conn())
	}
	return nil
}

func (tr *Transfer) Close() {
	tr.BaseImpl.Close()
	if tr.conn != nil {
		tr.Conn().Close()
	}
}
