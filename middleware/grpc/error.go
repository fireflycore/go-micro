package gm

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidationErrorToInvalidArgument() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		var ve *protovalidate.ValidationError
		if errors.As(err, &ve) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return resp, err
	}
}
