package etcd

import (
	"context"
	"fmt"
	clientV3 "go.etcd.io/etcd/client/v3"
	"shyIM/config"
	"shyIM/pkg/logger"
	"time"
)

// Registry  服务注册
type Registry struct {
	client        *clientV3.Client                        // etcd client
	leaseID       clientV3.LeaseID                        //租约ID
	keepAliveChan <-chan *clientV3.LeaseKeepAliveResponse // 租约 KeepAlive 相应chan
	key           string                                  // key
	val           string
}

func NewRegistry(key, value string, lease int64) error {
	client, err := clientV3.New(clientV3.Config{
		Endpoints:   config.GlobalConfig.Etcd.Endpoints,
		DialTimeout: time.Duration(config.GlobalConfig.Etcd.Timeout) * time.Second,
	})
	if err != nil {
		return err
	}
	regServer := &Registry{
		client: client,
		key:    key,
		val:    value,
	}
	if err = regServer.putKeyWithLease(lease); err != nil {
		return err
	}
	return nil
}

// putKeyWithLease 设置key和租约
func (r *Registry) putKeyWithLease(lease int64) error {
	leaseResp, err := r.client.Grant(context.TODO(), lease)
	if err != nil {
		return err
	}
	if _, err := r.client.Put(context.TODO(), r.key, r.val, clientV3.WithLease(leaseResp.ID)); err != nil {
		return err
	}
	leaseRespChan, err := r.client.KeepAlive(context.TODO(), leaseResp.ID)
	if err != nil {
		return err
	}

	r.leaseID = leaseResp.ID
	r.keepAliveChan = leaseRespChan
	return nil
}

// ListenLeaseRespChan 监听续租情况
func (r *Registry) ListenLeaseRespChan() {
	defer func(r *Registry) {
		err := r.close()
		if err != nil {
			logger.Slog.Error(fmt.Sprintf("%v\n", err))
		}
	}(r)

	for range r.keepAliveChan {
	}
}

func (r *Registry) close() error {
	if _, err := r.client.Revoke(context.TODO(), r.leaseID); err != nil {
		return err
	}
	logger.Slog.Info(fmt.Sprintf("撤销租约成功, leaseID:%d, Put key:%s,val:%s\n", r.leaseID, r.key, r.val))
	return r.client.Close()
}
