package tools

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os/user"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var defaultPort = "22"

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

func splitAddr(addr string) (host, path string) {
	sps := strings.Split(addr, ":")
	if len(sps) < 2 {
		path = sps[0]
	} else {
		host = sps[0]
		path = sps[1]
	}
	return
}

func ParseCopyParam(from, to string) (userName, host string, localPath string, remotePath string, upload bool, err error) {

	fromHost, fromPath := splitAddr(from)
	toHost, toPath := splitAddr(to)

	if fromHost != "" && toHost != "" {
		err = fmt.Errorf("both was remote hoost?")
	}
	if fromHost != "" {
		userName, host, err = GetParam(fromHost)
		upload = false
		remotePath = fromPath
		localPath = toPath
	}

	if toHost != "" {
		userName, host, err = GetParam(toHost)
		upload = true
		remotePath = toPath
		localPath = fromPath
	}

	return
}

func GetParam(addrStr string) (userName, addr string, err error) {
	sps := strings.Split(addrStr, "@")
	if len(sps) < 2 {
		user, err := user.Current()
		if err != nil {
			logrus.Fatalf(err.Error())
		}
		userName = user.Username
		addr = sps[0]
	} else {
		userName = sps[0]
		addr = sps[1]
	}
	return
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

	// // handle encrypted key
	// if x509.IsEncryptedPEMBlock(pemBlock) {
	// 	// decrypt PEM
	// 	pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
	// 	if err != nil {
	// 		return nil, fmt.Errorf("decrypting PEM block failed %v", err)
	// 	}

	// 	// get RSA, EC or DSA key
	// 	key, err := parsePemBlock(pemBlock)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	// generate signer instance from key
	// 	signer, err := ssh.NewSignerFromKey(key)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("creating signer from encrypted key failed %v", err)
	// 	}

	// 	return signer, nil
	// } else {
	// 	// generate signer instance from plain key
	// 	signer, err := ssh.ParsePrivateKey(pemBytes)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("parsing plain private key failed %v", err)
	// 	}

	// 	return signer, nil
	// }
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
