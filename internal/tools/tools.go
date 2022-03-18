package tools

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os/user"
	"strings"

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

func GetParam(addrStr string) (userName, addr, port string, err error) {
	sps := strings.Split(addrStr, "@")
	if len(sps) < 2 {
		user, err := user.Current()
		if err != nil {
			log.Fatalf(err.Error())
		}
		userName = user.Username
	} else {
		userName = sps[0]
		sps[0] = sps[1]
	}

	sps = strings.Split(sps[0], ":")
	if len(sps) < 2 {
		addr = sps[0]
		port = defaultPort
	} else {
		addr = sps[0]
		port = sps[1]
	}

	return
}

func CreateTmpListenAddress() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(1000)+10000)
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
