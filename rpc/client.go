package rpc

import (
	"github.com/fireflycore/go-micro/conf"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGrpcClient(bootstrapConf conf.BootstrapConf) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		bootstrapConf.GetGatewayEndpoint(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}
