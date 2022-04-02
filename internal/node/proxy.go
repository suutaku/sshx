package node

import (
	"context"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/internal/proto"
)

type ProxyRepo struct {
	proto.ProxyInfo
	ConnetionKey string
	cancel       *context.CancelFunc
}

type ProxyManager struct {
	repo map[string]*ProxyRepo
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		repo: make(map[string]*ProxyRepo),
	}
}

// diffrent host cannot use same port
func (pm *ProxyManager) Validation(info *ProxyRepo) error {
	for k, v := range pm.repo {
		if k == info.Host && v.ProxyPort == info.ProxyPort {
			return fmt.Errorf("proxy port %d already in use", v.ProxyPort)
		}
	}
	return nil
}

func (pm *ProxyManager) AddProxy(info *ProxyRepo) error {
	logrus.Debug("add proxy for ", info.Host)
	err := pm.Validation(info)
	if err != nil {
		return err
	}
	if pm.repo[info.Host] != nil {
		pm.repo[info.Host].ConnectionNumber++
	} else {
		pm.repo[info.Host] = info
	}
	return nil
}

func (pm *ProxyManager) RemoveProxy(host string) {
	if pm.repo[host] == nil {
		return
	}
	logrus.Debug("cancel proxy listening")
	(*pm.repo[host].cancel)()
	delete(pm.repo, host)
}

func (pm *ProxyManager) GetConnectionKeys(host string) []string {
	ret := make([]string, 0)
	if host == "" {
		for _, v := range pm.repo {
			ret = append(ret, v.ConnetionKey)
		}
		return ret
	}
	if pm.repo[host] != nil {
		ret = append(ret, pm.repo[host].ConnetionKey)
	}
	return ret
}

func (pm *ProxyManager) GetProxyInfos(host string) proto.ListDestroyResponse {
	ret := proto.ListDestroyResponse{}
	if host == "" {
		for _, v := range pm.repo {
			ret.List = append(ret.List, &v.ProxyInfo)
		}
		return ret
	}
	if pm.repo[host] != nil {
		ret.List = append(ret.List, &pm.repo[host].ProxyInfo)
	}
	return ret
}

func (node *Node) Proxy(ctx context.Context, req proto.ConnectRequest) {
	tmpListenAddr := fmt.Sprintf(":%d", req.ProxyPort)
	l, err := net.Listen("tcp", tmpListenAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	logrus.Println("Proxy listen on:", tmpListenAddr)
	runing := true
	go func(runing *bool) {
		for *runing {
			sock, err := l.Accept()
			if err != nil {
				continue
			}
			go node.Connect(ctx, sock, req)
		}
		logrus.Println("proxy accept loop canceled")
	}(&runing)

	<-ctx.Done()
	runing = false
	logrus.Println("proxy canceled")
	l.Close()

}
