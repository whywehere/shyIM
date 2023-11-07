package middlewares

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"shyIM/pkg/utils"
)

func AuthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("token")
		userClaims, err := utils.AnalyseToken(token)
		if err != nil {
			c.Abort()
			c.JSON(http.StatusOK, gin.H{
				"code": -1,
				"msg":  "用户认证未通过",
			})
			return
		}
		c.Set("user_claims", userClaims)
		c.Next()
	}
}
