package rpc

import (
	"google.golang.org/grpc"
	"shyIM/pkg/logger"
	"shyIM/pkg/protocol/pb"
)

var (
	ConnServerClient pb.ConnectClient
)

// GetServerClient 获取 grpc 连接
func GetServerClient(addr string) pb.ConnectClient {
	client, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		logger.Slog.Error("Grpc Client Dial Failed ", "[ERROR]", err)
		panic(err)
	}
	ConnServerClient = pb.NewConnectClient(client)
	return ConnServerClient
}
