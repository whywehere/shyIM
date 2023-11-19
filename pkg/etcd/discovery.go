package etcd

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientV3 "go.etcd.io/etcd/client/v3"
	"shyIM/config"
	"shyIM/pkg/logger"
	"sync"
	"time"
)

// Discovery 服务发现
type Discovery struct {
	client    *clientV3.Client //    etcd client
	serverMap sync.Map
}

func NewDiscovery() (*Discovery, error) {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:   config.GlobalConfig.Etcd.Endpoints,
		DialTimeout: time.Duration(config.GlobalConfig.Etcd.Timeout) * time.Second,
	})
	if err != nil {
		logger.Slog.Error("Failed to create etcd client", "[ERROR]", err)
		return nil, err
	}
	logger.Slog.Info("Creating Discovery succeeded")
	return &Discovery{client: client}, nil
}

func (d *Discovery) WatchServices(prefix string) {
	// 获取以prefix开头的services(rpc address)
	resp, err := d.client.Get(context.TODO(), prefix, clientV3.WithPrefix())
	if err != nil {
		logger.Slog.Error("Failed to Get Discovery", "err", err)
		return
	}
	// 将所有services 存入 serverMap
	for i := range resp.Kvs {
		if v := resp.Kvs[i]; v != nil {
			d.serverMap.Store(string(v.Key), string(v.Value))
		}
	}
	d.watcher(prefix)
	return
}

func (d *Discovery) watcher(prefix string) {
	watchChan := d.client.Watch(context.TODO(), prefix, clientV3.WithPrefix())
	for wResp := range watchChan {
		for _, event := range wResp.Events {
			switch event.Type {
			case mvccpb.PUT:
				d.serverMap.Store(string(event.Kv.Key), string(event.Kv.Value))
			case mvccpb.DELETE:
				d.serverMap.Delete(string(event.Kv.Key))
			}

		}
	}
}

func (d *Discovery) Close() error {
	return d.client.Close()
}

func (d *Discovery) GetServices() []string {
	addrs := make([]string, 0)
	d.serverMap.Range(func(key, value interface{}) bool {
		addrs = append(addrs, value.(string))
		fmt.Println(value.(string))
		return true
	})

	return addrs
}
