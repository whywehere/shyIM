package router

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"shyIM/config"
	"shyIM/pkg/logger"
	"shyIM/service/ws"
	"syscall"
	"time"
)

var upGrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSRouter websocket 路由
func WSRouter() {
	// connServerManager 单例
	connServerManager := ws.GetServer()

	// 开启worker工作池
	connServerManager.StartWorkerPool()

	// 开启心跳超时检测
	heartbeatChecker := ws.NewHeartbeatChecker(time.Second*time.Duration(config.GlobalConfig.APP.HeartbeatInterval), connServerManager)

	go heartbeatChecker.Start()

	r := gin.Default()

	gin.SetMode(gin.ReleaseMode)

	pprof.Register(r)

	var connID uint64

	r.GET("/ws", func(c *gin.Context) {
		// 升级协议  http -> websocket
		WsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Slog.Error("[WebSocket upGrade Failed]", "err", err)
			return
		}

		// 初始化连接
		conn := ws.NewConnection(connServerManager, WsConn, connID)
		connID++

		// 开启读写线程
		go conn.Start()
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.WebSocketServerPort),
		Handler: r,
	}

	go func() {
		fmt.Println("websocket 启动：", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 关闭服务
	connServerManager.Stop()
	heartbeatChecker.Stop()

	// 5s 超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown: ", err)
	}

	log.Println("Server exiting")
}
