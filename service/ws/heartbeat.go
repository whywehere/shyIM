package ws

import (
	"shyIM/pkg/logger"
	"time"
)

// HeartbeatChecker 心跳检测
type HeartbeatChecker struct {
	interval time.Duration // 心跳检测时间间隔

	quit chan struct{} // 退出信号

	server *Server // 所属服务端
}

func NewHeartbeatChecker(interval time.Duration, s *Server) *HeartbeatChecker {
	return &HeartbeatChecker{
		interval: interval,
		quit:     make(chan struct{}, 1),
		server:   s,
	}
}

// Start 启动心跳检测
func (h *HeartbeatChecker) Start() {
	ticker := time.NewTicker(h.interval)
	for {
		select {
		case <-ticker.C:
			h.check()
		case <-h.quit:
			ticker.Stop()
			return
		}
	}
}

// Stop 停止心跳检测
func (h *HeartbeatChecker) Stop() {
	h.quit <- struct{}{}
}

// check 超时检测
func (h *HeartbeatChecker) check() {
	logger.Slog.Info("[heartbeat check started]", "Time", time.Now().Format("2006-01-02 15:04:05"))
	// 已验证的连接
	conns := h.server.GetAllConn()
	for _, conn := range conns {
		if !conn.IsAlive() {
			conn.Stop()
		}
	}
}
