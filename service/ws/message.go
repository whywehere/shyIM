package ws

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"shyIM/common"
	"shyIM/config"
	"shyIM/model"
	"shyIM/model/cache"
	"shyIM/pkg/etcd"
	"shyIM/pkg/logger"
	"shyIM/pkg/mq"
	"shyIM/pkg/protocol/pb"
	"shyIM/pkg/rpc"
	"shyIM/service"
	"time"
)

// GetOutputMsg 组装出下行消息
func GetOutputMsg(cmdType pb.CmdType, code int32, message proto.Message) ([]byte, error) {
	output := &pb.Output{
		Type:    cmdType,
		Code:    code,
		CodeMsg: common.GetErrorMessage(uint32(code), ""),
		Data:    nil,
	}

	if message != nil {
		msgBytes, err := proto.Marshal(message)
		if err != nil {
			logger.Slog.Error("[GetOutputMsg] message marshal failed", "[ERROR]", err)
			return nil, err
		}
		output.Data = msgBytes
	}

	bytes, err := proto.Marshal(output)
	if err != nil {
		logger.Slog.Error("[GetOutputMsg] message marshal failed", "[ERROR]", err)
		return nil, err
	}
	return bytes, nil
}

// SendToUser 发送消息到好友
func SendToUser(msg *pb.Message, userId uint64) (uint64, error) {
	// 获取接受者 seqId
	seq, err := service.GetUserNextSeq(userId)
	if err != nil {
		fmt.Println("[GetOutputMsg] 获取 seq 失败,err:", err)
		return 0, err
	}
	msg.Seq = seq

	// 发给MQ
	if err = mq.MessageMQ.Publish(model.MessageToProtoMarshal(&model.Message{
		UserID:      userId,
		SenderID:    msg.SenderId,
		SessionType: int8(msg.SessionType),
		ReceiverId:  msg.ReceiverId,
		MessageType: int8(msg.MessageType),
		Content:     msg.Content,
		Seq:         seq,
		SendTime:    time.UnixMilli(msg.SendTime),
	})); err != nil {
		logger.Slog.Error("[GetOutputMsg] mq.MessageMQ.Publish(messageBytes) 失败", "[ERROR]", err)
		return 0, err
	}

	// 如果发给自己的，只落库不进行发送
	if userId == msg.SenderId {
		return seq, nil
	}

	// 组装消息
	bytes, err := GetOutputMsg(pb.CmdType_CT_Message, int32(common.OK), &pb.PushMsg{Msg: msg})
	if err != nil {
		logger.Slog.Error("[GetOutputMsg] message marshal failed", "[ERROR]", err)
		return 0, err
	}

	// 进行推送
	return 0, Send(userId, bytes)
}

// Send 消息转发
// 是否在线 ---否---> 不进行推送
//    |
//    是
//    ↓
//  是否在本地 --否--> RPC 调用
//    |
//    是
//    ↓
//  消息发送

func Send(receiverId uint64, bytes []byte) error {
	// 查询是否在线
	rpcAddr, err := cache.GetUserOnline(receiverId)
	if err != nil {
		return err
	}

	// 不在线
	if rpcAddr == "" {
		logger.Slog.Warn("[消息处理]，用户不在线", "receiverId", receiverId)
		return nil
	}
	logger.Slog.Info("[消息处理]，用户在线", "rpcAddr", rpcAddr)

	// 查询是否在本地
	conn := ConnManager.GetConn(receiverId)
	if conn != nil {
		// 发送本地消息
		conn.SendMsg(receiverId, bytes)
		logger.Slog.Info("[Send]， 发送本地消息给用户 ", "receiverId", receiverId)
		return nil
	}

	// rpc 调用
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = rpc.GetServerClient(rpcAddr).DeliverMessage(ctx, &pb.DeliverMessageReq{
		ReceiverId: receiverId,
		Data:       bytes,
	})

	if err != nil {
		logger.Slog.Error("[Send] DeliverMessage Failed", "[ERROR]", err)
		return err
	}

	return nil
}

// SendToGroup 发送消息到群
func SendToGroup(msg *pb.Message) error {
	// 获取群成员信息
	userIds, err := service.GetGroupUser(msg.ReceiverId)
	if err != nil {
		logger.Slog.Error("[SendToGroup] 查询失败", "err", err)
		return err
	}

	// userId set
	m := make(map[uint64]struct{}, len(userIds))
	for _, userId := range userIds {
		m[userId] = struct{}{}
	}

	// 检查当前用户是否属于该群
	if _, ok := m[msg.SenderId]; !ok {
		fmt.Printf("发送消息者: %v 不属于该群: %v\n", msg.ReceiverId, msg.SenderId)
		return nil
	}

	// 自己不再进行推送
	delete(m, msg.SenderId)

	sendUserIds := make([]uint64, 0, len(m))
	for userId := range m {
		sendUserIds = append(sendUserIds, userId)
	}

	// 批量获取 seqId
	sequences, err := service.GetUserNextSeqBatch(sendUserIds)
	if err != nil {
		logger.Slog.Error("[批量获取 sequences 失败]", "err", err)
		return err
	}

	//  k:userid v:该userId的seq
	sendUserSet := make(map[uint64]uint64, len(sequences))
	for i, userId := range sendUserIds {
		sendUserSet[userId] = sequences[i]
	}

	// 创建 Message 对象
	messages := make([]*model.Message, 0, len(m))

	for userId, seq := range sendUserSet {
		messages = append(messages, &model.Message{
			UserID:      userId,
			SenderID:    msg.SenderId,
			SessionType: int8(msg.SessionType),
			ReceiverId:  msg.ReceiverId,
			MessageType: int8(msg.MessageType),
			Content:     msg.Content,
			Seq:         seq,
			SendTime:    time.UnixMilli(msg.SendTime),
		})
	}

	// 发给MQ
	err = mq.MessageMQ.Publish(model.MessageToProtoMarshal(messages...))
	if err != nil {
		fmt.Println("[消息处理] 群聊消息发送 MQ 失败,err:", err)
		return err
	}

	// 组装消息，进行推送
	userId2Msg := make(map[uint64][]byte, len(m))
	for userId, seq := range sendUserSet {
		msg.Seq = seq
		bytes, err := GetOutputMsg(pb.CmdType_CT_Message, int32(common.OK), &pb.PushMsg{Msg: msg})
		if err != nil {
			fmt.Println("[消息处理] GetOutputMsg Marshal error,err:", err)
			return err
		}
		userId2Msg[userId] = bytes
	}

	// 获取全部网关服务，进行消息推送
	services := etcd.DiscoverySer.GetServices()
	local := fmt.Sprintf("%s:%s", config.GlobalConfig.APP.IP, config.GlobalConfig.APP.RPCPort)
	for _, addr := range services {
		// 如果是本机，进行本地推送
		if local == addr {
			GetServer().SendMessageAll(userId2Msg)
		} else {
			// 如果不是本机，进行远程 RPC 调用
			_, err = rpc.GetServerClient(addr).DeliverMessageAll(context.Background(), &pb.DeliverMessageAllReq{
				ReceiverId_2Data: userId2Msg,
			})

			if err != nil {
				logger.Slog.Error("[DeliverMessageAll]", "err", err)
				return err
			}
		}
	}

	return nil
}
