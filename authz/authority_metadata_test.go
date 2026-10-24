package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// fixedServiceAuthorityProvider 用固定凭证模拟当前服务的身份提供器。
type fixedServiceAuthorityProvider string

func (p fixedServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return string(p), nil
}

// TestStreamClientFreshMetadata 验证流式调用重新生成元数据，且缺少服务凭证时不发起调用。
func TestStreamClientFreshMetadata(t *testing.T) {
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(
		constant.UserAuthority, "user", constant.AuthzSign, "old"), service.BuildContextOptions{
		ServiceAppId: "B", ServiceMethod: "GRPC", ServicePath: "/B/Get",
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	streamer := func(ctx context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, method string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
		called = true
		md, _ := metadata.FromOutgoingContext(ctx)
		if method != "/C/Stream" || md.Get(constant.ServiceAuthority)[0] != "B-token" ||
			md.Get(constant.UserAuthority)[0] != "user" || md.Get(constant.InvokePath)[0] != "/B/Get" || len(md.Get(constant.AuthzSign)) != 0 {
			t.Fatal("incorrect stream metadata")
		}
		return nil, nil
	}
	_, err = NewServiceAuthorityStreamClientInterceptor(fixedServiceAuthorityProvider("B-token"))(ctx, &grpc.StreamDesc{}, nil, "/C/Stream", streamer)
	if err != nil || !called {
		t.Fatal(err)
	}
	called = false
	_, err = NewServiceAuthorityStreamClientInterceptor(nil)(ctx, &grpc.StreamDesc{}, nil, "/C/Stream", streamer)
	if err == nil || called {
		t.Fatal("stream downgraded missing provider")
	}
}

// failingServiceAuthorityProvider 模拟服务凭证获取失败。
type failingServiceAuthorityProvider struct {
	err error // 需要原样向调用方传递的获取错误。
}

func (p failingServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return "", p.err
}

// TestOutgoingPrivateAuthorityAndNoDowngrade 验证私有用户凭证继承与服务身份不可降级。
func TestOutgoingPrivateAuthorityAndNoDowngrade(t *testing.T) {
	md := metadata.Pairs(constant.UserAuthority, "user", constant.AuthzSign, "old", constant.ServiceAuthority, "upstream", constant.AppVersion, "1")
	ctx, err := service.NewIncomingContext(context.Background(), md, service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "GRPC", ServicePath: "/B/Get"})
	if err != nil {
		t.Fatal(err)
	}
	target := service.OutgoingTarget{TargetMethod: "GRPC", TargetPath: "/C/Get"}
	out, err := PrepareOutgoingAuthorityMetadata(ctx, target, fixedServiceAuthorityProvider("B-token"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Get(constant.UserAuthority)[0] != "user" || out.Get(constant.ServiceAuthority)[0] != "B-token" || out.Get(constant.InvokePath)[0] != "/B/Get" ||
		len(out.Get(constant.AuthzSign)) != 0 {
		t.Fatal(out)
	}
	// 缺少提供器、返回空值和获取失败都必须拒绝，不能退化成用户直调。
	for _, p := range []ServiceAuthorityProvider{nil, fixedServiceAuthorityProvider(""), failingServiceAuthorityProvider{errors.New("unavailable")}} {
		if _, err := PrepareOutgoingAuthorityMetadata(ctx, target, p); err == nil {
			t.Fatal("downgraded provider failure")
		}
	}
	// 任意写入出站 metadata 的凭证不等同于框架保管的入站用户凭证。
	raw := metadata.NewOutgoingContext(context.Background(), md)
	out, err = PrepareOutgoingAuthorityMetadata(raw, target, fixedServiceAuthorityProvider("B-token"))
	if err != nil || len(out.Get(constant.UserAuthority)) != 0 {
		t.Fatal("inherited arbitrary outgoing credentials")
	}
}

// TestGRPCOutgoingRejectsNonASCII 在发送前拒绝 gRPC 无法编码的观测值，不发明隐式转义协议。
func TestGRPCOutgoingRejectsNonASCII(t *testing.T) {
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.ClientName, "中文客户端"),
		service.BuildContextOptions{ServiceAppId: "B", ServiceMethod: "POST", ServicePath: "/entry"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = NewServiceAuthorityUnaryClientInterceptor(fixedServiceAuthorityProvider("B-token"))(
		ctx, "/C/Get", nil, nil, nil,
		func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
			called = true
			return nil
		},
	)
	if !errors.Is(err, service.ErrIncomingContextInvalid) || called {
		t.Fatalf("unsupported metadata reached transport: called=%v err=%v", called, err)
	}
}
