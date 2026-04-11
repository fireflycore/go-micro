package rpc

import (
	"github.com/fireflycore/go-micro/config"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGrpcClient(bootstrapConf config.BootstrapConfig) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		bootstrapConf.GetGatewayEndpoint(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}
