package rpc_server

import (
	"context"
	"google.golang.org/protobuf/types/known/emptypb"
	"shyIM/pkg/protocol/pb"
)

type ConnectServer struct {
	pb.UnsafeConnectServer // 禁止向前兼容
}

func (*ConnectServer) DeliverMessage(ctx context.Context, req *pb.DeliverMessageReq) (*emptypb.Empty, error) {
	resp := &emptypb.Empty{}
	return resp, nil
}
