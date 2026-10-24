package invocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fireflycore/go-micro/authz"
	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// testDialer 隔离真实网络，分别模拟连接成功与拨号失败。
type testDialer struct {
	conn *grpc.ClientConn // 供测试调用函数接收的连接占位对象。
	err  error            // 预设拨号错误；用于验证错误传递。
}

func (d testDialer) Dial(context.Context, *DNS) (*grpc.ClientConn, error) { return d.conn, d.err }
func (d testDialer) Close() error                                         { return nil }

// fixedServiceAuthorityProvider 返回当前服务的固定测试凭证。
type fixedServiceAuthorityProvider string

func (p fixedServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return string(p), nil
}

// failingServiceAuthorityProvider 模拟当前服务无法获取凭证。
type failingServiceAuthorityProvider struct {
	err error // 获取凭证失败时返回的预设错误。
}

func (p failingServiceAuthorityProvider) ServiceAuthority(context.Context) (string, error) {
	return "", p.err
}

// privateContext 通过框架入站路径保管用户凭证，旧签名仅作为待清理的输入。
func privateContext(t *testing.T) context.Context {
	t.Helper()
	ctx, err := service.NewIncomingContext(context.Background(), metadata.Pairs(constant.UserAuthority, "user-token", constant.AuthzSign, "old"),
		service.BuildContextOptions{ServiceAppId: "A", ServiceMethod: "POST", ServicePath: "/a"})
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// TestInvokePrivateCredentialsDeadlineAndCancellation 验证凭证重建不破坏原生超时和取消。
func TestInvokePrivateCredentialsDeadlineAndCancellation(t *testing.T) {
	u := NewUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, 200*time.Millisecond).WithServiceAuthorityProvider(fixedServiceAuthorityProvider("A-token"))
	called := false
	u.InvokeFunc = func(ctx context.Context, _ *grpc.ClientConn, _ string, _, _ any, _ ...grpc.CallOption) error {
		called = true
		md, _ := metadata.FromOutgoingContext(ctx)
		if md.Get(constant.UserAuthority)[0] != "user-token" || md.Get(constant.ServiceAuthority)[0] != "A-token" || len(md.Get(constant.AuthzSign)) != 0 {
			t.Fatal(md)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 200*time.Millisecond {
			t.Fatal("lost deadline")
		}
		return nil
	}
	if err := u.Invoke(privateContext(t), &DNS{Service: "B"}, "/B/Get", nil, nil); err != nil || !called {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(privateContext(t))
	cancel()
	if err := u.Invoke(ctx, nil, "/B/Get", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// TestInvokeErrors 覆盖配置缺失、方法为空以及凭证获取和拨号失败的错误路径。
func TestInvokeErrors(t *testing.T) {
	u := NewUnaryInvoker(testDialer{}, 0)
	if u.Timeout != DefaultInvokeTimeout {
		t.Fatal("wrong default")
	}
	if err := u.Invoke(context.Background(), nil, "/B/Get", nil, nil); !errors.Is(err, authz.ErrServiceAuthorityFetchMissing) {
		t.Fatal(err)
	}
	if err := u.Invoke(context.Background(), nil, "", nil, nil); err != ErrInvokeMethodEmpty {
		t.Fatal(err)
	}
	var absent *UnaryInvoker
	if err := absent.Invoke(context.Background(), nil, "/B/Get", nil, nil); err != ErrInvokerDialerIsNil {
		t.Fatal(err)
	}
	expected := errors.New("dial failed")
	u.Dialer = testDialer{err: expected}
	u.ServiceAuthorityProvider = fixedServiceAuthorityProvider("A-token")
	if err := u.Invoke(context.Background(), nil, "/B/Get", nil, nil); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	u.ServiceAuthorityProvider = failingServiceAuthorityProvider{expected}
	if err := u.Invoke(context.Background(), nil, "/B/Get", nil, nil); !errors.Is(err, expected) {
		t.Fatal(err)
	}
}

// TestBootstrapIsMethodBoundAndCredentialFree 验证凭证引导调用只开放指定方法且不携带身份元数据。
func TestBootstrapIsMethodBoundAndCredentialFree(t *testing.T) {
	b := NewBootstrapUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, time.Second, "/auth/Issue")
	b.InvokeFunc = func(ctx context.Context, _ *grpc.ClientConn, _ string, _, _ any, _ ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md) != 0 {
			t.Fatal(md)
		}
		return nil
	}
	if err := b.Invoke(privateContext(t), nil, "/auth/Issue", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Invoke(privateContext(t), nil, "/B/Get", nil, nil); err == nil {
		t.Fatal("unbound bootstrap")
	}
}

// TestNewOutgoingCallContextCopiesAndPreservesParent 验证元数据独立复制，取消仍沿父上下文传递。
func TestNewOutgoingCallContextCopiesAndPreservesParent(t *testing.T) {
	p, stop := context.WithCancel(context.Background())
	md := metadata.Pairs("key", "old")
	ctx, cancel := NewOutgoingCallContext(p, md, time.Second)
	defer cancel()
	md.Set("key", "new")
	out, _ := metadata.FromOutgoingContext(ctx)
	if out.Get("key")[0] != "old" {
		t.Fatal("shared metadata")
	}
	stop()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("lost cancellation")
	}
}

// TestOutgoingCallsRequireExplicitContext 防止 nil 被静默替换，后台调用仍可显式使用 Background。
func TestOutgoingCallsRequireExplicitContext(t *testing.T) {
	invoke := func(context.Context, *grpc.ClientConn, string, any, any, ...grpc.CallOption) error { return nil }
	u := NewUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, time.Second).
		WithServiceAuthorityProvider(fixedServiceAuthorityProvider("A-token"))
	u.InvokeFunc = invoke
	b := NewBootstrapUnaryInvoker(testDialer{conn: &grpc.ClientConn{}}, time.Second, "/auth/Issue")
	b.InvokeFunc = invoke
	for _, tc := range []struct {
		name string                      // 三种出站入口遵守相同的父 context 契约。
		call func(context.Context) error // 只替换父 context，其余调用依赖保持有效。
	}{
		{name: "unary", call: func(ctx context.Context) error {
			return u.Invoke(ctx, nil, "/B/Get", nil, nil)
		}},
		{name: "bootstrap", call: func(ctx context.Context) error {
			return b.Invoke(ctx, nil, "/auth/Issue", nil, nil)
		}},
		{name: "outgoing_context", call: func(ctx context.Context) error {
			out, cancel := NewOutgoingCallContext(ctx, metadata.MD{}, time.Second)
			defer cancel()
			return out.Err()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(context.Background()); err != nil {
				t.Fatalf("explicit background context failed: %v", err)
			}
			defer func() {
				if recover() == nil {
					t.Fatal("nil context was silently accepted")
				}
			}()
			_ = tc.call(nil)
		})
	}
}
