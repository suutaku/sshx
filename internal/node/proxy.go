package node

import (
	"context"
	"fmt"
	"log"

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
	log.Println("add proxy for ", info.Host)
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
	log.Println("cancel proxy listening")
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
