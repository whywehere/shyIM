package ws

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"shyIM/common"
	"shyIM/config"
	"shyIM/model"
	"shyIM/model/cache"
	"shyIM/pkg/logger"
	"shyIM/pkg/protocol/pb"
	"shyIM/pkg/utils"
)

// Handler 路由函数
type Handler func()

// Req 请求
type Req struct {
	conn *Conn   // 连接
	data []byte  // 客户端发送的请求数据
	f    Handler // 该请求需要执行的路由函数
}

func (r *Req) Login() {
	// 检查用户是否已登录 只能防止同一个连接多次调用 Login
	if r.conn.GetUserId() != 0 {
		fmt.Printf(fmt.Sprintf("User[%v] has logged in\n", r.conn.GetUserId))
		return
	}

	loginMsg := new(pb.LoginMsg)
	err := proto.Unmarshal(r.data, loginMsg)
	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}
	// 登录校验
	userClaims, err := utils.AnalyseToken(string(loginMsg.Token))
	if err != nil {
		logger.Slog.Error("[AnalyseToken failed]", "err", err)
		return
	}

	// 检查用户是否已经在其他连接登录
	onlineAddr, err := cache.GetUserOnline(userClaims.UserId)
	if onlineAddr != "" {
		// TODO 更友好的提示
		fmt.Printf(fmt.Sprintf("User[%v] has logged in another device\n", r.conn.GetUserId))
		r.conn.Stop()
		return
	}

	// Redis 存储用户数据 k: userId,  v: grpc地址，方便用户能直接通过这个地址进行 rpc 方法调用
	grpcServerAddr := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
	err = cache.SetUserOnline(userClaims.UserId, grpcServerAddr)
	if err != nil {
		logger.Slog.Error(err.Error())
		fmt.Println("Internal System Error")
		return
	}

	// 设置 user_id
	r.conn.SetUserId(userClaims.UserId)

	// 加入到 connMap 中
	r.conn.server.AddConn(userClaims.UserId, r.conn)

	// 回复ACK
	bytes, err := GetOutputMsg(pb.CmdType_CT_ACK, int32(common.OK), &pb.ACKMsg{Type: pb.ACKType_AT_Login})
	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}

	// 回复发送 Login 请求的客户端
	r.conn.SendMsg(userClaims.UserId, bytes)
}

func (r *Req) Heartbeat() {
	// TODO 更新当前用户状态，不做回复
}

/*
MessageHandler 消息处理，处理客户端发送给服务端的消息
A客户端发送消息给服务端，服务端收到消息处理后发给B客户端
包括：单聊、群聊
*/
func (r *Req) MessageHandler() {

	msg := new(pb.UpMsg)
	err := proto.Unmarshal(r.data, msg)

	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}

	// 实现消息可靠性
	if !r.conn.CompareAndIncrClientID(msg.ClientId) {
		return
	}

	if msg.Msg.SenderId != r.conn.GetUserId() {
		return
	}

	// 单聊不能发给自己
	if msg.Msg.SessionType == pb.SessionType_ST_Single && msg.Msg.ReceiverId == r.conn.GetUserId() {
		fmt.Println("[消息处理] 不能将消息发送给自己")
		return
	}

	// 给自己发一份，消息落库但是不推送
	seq, err := SendToUser(msg.Msg, msg.Msg.SenderId)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 单聊、群聊
	switch msg.Msg.SessionType {
	case pb.SessionType_ST_Single:
		_, err = SendToUser(msg.Msg, msg.Msg.ReceiverId)
	case pb.SessionType_ST_Group:
		err = SendToGroup(msg.Msg)
	default:
		return
	}

	if err != nil {
		fmt.Println("[消息发送] 系统错误")
		return
	}

	// 回复发送上行消息的客户端 ACK
	ackBytes, err := GetOutputMsg(pb.CmdType_CT_ACK, common.OK, &pb.ACKMsg{
		Type:     pb.ACKType_AT_Up,
		ClientId: msg.ClientId, // 回复客户端，当前已 ACK 的消息
		Seq:      seq,          // 回复客户端当前其 seq
	})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	// 回复发送 Message 请求的客户端 A
	r.conn.SendMsg(msg.Msg.SenderId, ackBytes)
}

// Sync  消息同步，拉取离线消息
func (r *Req) Sync() {

	msg := new(pb.SyncInputMsg)

	err := proto.Unmarshal(r.data, msg)
	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}

	// 根据 seq 查询，得到比 seq 大的用户消息
	messages, hasMore, err := model.ListByUserIdAndSeq(r.conn.GetUserId(), msg.Seq, model.MessageLimit)
	if err != nil {
		fmt.Println("离线消息同步失败")
		logger.Slog.Error(err.Error())
		return
	}
	pbMessage := model.MessagesToPB(messages)

	ackBytes, err := GetOutputMsg(pb.CmdType_CT_Sync, int32(common.OK), &pb.SyncOutputMsg{
		Messages: pbMessage,
		HasMore:  hasMore,
	})
	if err != nil {
		logger.Slog.Error(err.Error())
		return
	}
	// 回复
	r.conn.SendMsg(r.conn.GetUserId(), ackBytes)
}
