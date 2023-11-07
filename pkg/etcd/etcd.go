package etcd

import (
	"fmt"
	"shyIM/common"
	"shyIM/config"
	"shyIM/pkg/logger"
	"time"
)

var (
	DiscoverySer *Discovery
)

func Start() {
	hostPort := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
	if err := NewRegistry(common.EtcdServerList+hostPort, hostPort, 5); err != nil {
		logger.Slog.Error("Failed to NewRegistry", "[ERROR]", err)
		return
	}

	time.Sleep(100 * time.Millisecond)

	DiscoverySer, _ := NewDiscovery()

	// 阻塞监听
	DiscoverySer.WatchServices(common.EtcdServerList)

	return
}
