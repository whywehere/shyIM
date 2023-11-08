package router

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"shyIM/pkg/logger"
	"shyIM/pkg/middlewares"
	"shyIM/service"
)

func HTTPRouter() {
	r := gin.Default()

	gin.SetMode(gin.ReleaseMode)

	// 用户注册
	r.POST("/register", service.RegisterHandler)

	// 用户登录
	r.POST("/login", service.LoginHandler)

	auth := r.Group("", middlewares.AuthCheck())
	{
		// 添加好友
		auth.POST("/friend/add", service.AddFriendHandler)

		// 创建群聊
		auth.POST("/group/create", service.CreateGroupHandler)

		// 获取群成员列表
		auth.GET("/group_user/list", service.GroupUserList)
	}
	if err := r.Run(":8081"); err != nil && err != http.ErrServerClosed {
		logger.Slog.Error("gin engine run error", "[ERROR]", err)
	}
}
