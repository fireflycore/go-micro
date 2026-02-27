package rpc

import (
	"github.com/fireflycore/go-micro/conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewRemoteServiceGrpcClient(bootstrapConf conf.BootstrapConf) (*grpc.ClientConn, error) {
	return grpc.NewClient(bootstrapConf.GetGatewayEndpoint(), grpc.WithTransportCredentials(insecure.NewCredentials()))
}
