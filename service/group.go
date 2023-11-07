package service

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"shyIM/model"
	"shyIM/model/cache"
	"shyIM/pkg/utils"
)

// CreateGroupHandler 创建群聊
func CreateGroupHandler(c *gin.Context) {
	// 参数校验
	name := c.PostForm("name")
	idsStr := c.PostFormArray("ids") // 群成员 id，不包括群创建者
	if name == "" || len(idsStr) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "参数不正确",
		})
		return
	}
	ids := make([]uint64, 0, len(idsStr)+1)
	for i := range idsStr {
		ids = append(ids, utils.StrToUint64(idsStr[i]))
	}
	// 获取用户信息
	uc := c.MustGet("user_claims").(*utils.UserClaims)
	ids = append(ids, uc.UserId)

	// 获取 ids 用户信息
	ids, err := model.GetUserIdByIds(ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "系统错误:" + err.Error(),
		})
		return
	}

	// 创建群组
	group := &model.Group{
		Name:    name,
		OwnerID: uc.UserId,
	}
	err = model.CreateGroup(group, ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "系统错误:" + err.Error(),
		})
		return
	}
	// 将群成员信息更新到 Redis
	err = cache.SetGroupUser(group.ID, ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "系统错误:" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "群组创建成功",
		"data": gin.H{
			"id": utils.Uint64ToStr(group.ID),
		},
	})
}

// GroupUserList 获取群成员列表
func GroupUserList(c *gin.Context) {
	// 参数校验
	groupIdStr := c.Query("group_id")
	groupId := utils.StrToUint64(groupIdStr)
	if groupId == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "参数不正确",
		})
		return
	}
	// 获取用户信息
	uc := c.MustGet("user_claims").(*utils.UserClaims)

	// 验证用户是否属于该群
	isBelong, err := model.IsBelongToGroup(uc.UserId, groupId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "系统错误:" + err.Error(),
		})
		return
	}
	if !isBelong {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "用户不属于该群",
		})
		return
	}

	// 获取群成员id列表
	ids, err := GetGroupUser(groupId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": -1,
			"msg":  "系统错误:" + err.Error(),
		})
		return
	}
	var idsStr []string
	for _, id := range ids {
		idsStr = append(idsStr, utils.Uint64ToStr(id))
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "请求成功",
		"data": gin.H{
			"ids": idsStr,
		},
	})
}

// GetGroupUser 获取群成员
// 从缓存中获取，如果缓存中没有，获取后加入缓存
func GetGroupUser(groupId uint64) ([]uint64, error) {
	userIds, err := cache.GetGroupUser(groupId)
	if err != nil {
		return nil, err
	}
	if len(userIds) != 0 {
		return userIds, nil
	}

	userIds, err = model.GetGroupUserIdsByGroupId(groupId)
	if err != nil {
		return nil, err
	}
	err = cache.SetGroupUser(groupId, userIds)
	if err != nil {
		return nil, err
	}

	return userIds, nil
}
