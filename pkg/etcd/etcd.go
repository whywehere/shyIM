package etcd

import (
	"fmt"
	"shyIM/common"
	"shyIM/config"
	"time"
)

var (
	DiscoverySer *Discovery
)

func InitEtcd() (err error) {
	hostPort := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
	if err = NewRegistry(common.EtcdServerList+hostPort, hostPort, 5); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	DiscoverySer, err = NewDiscovery()
	if err != nil {
		return err
	}

	// 阻塞监听
	if err := DiscoverySer.WatchServices(common.EtcdServerList); err != nil {
		return err
	}
	return
}
