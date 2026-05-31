package authz

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewServiceAuthorityProvider_Disabled(t *testing.T) {
	provider, err := NewServiceAuthorityProvider(nil, nil)
	if err != nil {
		t.Fatalf("expected disabled provider without error, got %v", err)
	}
	if provider != nil {
		t.Fatalf("expected nil provider when config is disabled")
	}
}

func TestNewServiceAuthorityProvider_RequiresFetchWhenEnabled(t *testing.T) {
	_, err := NewServiceAuthorityProvider(&ServiceAuthorityConfig{Enabled: true}, nil)
	if !errors.Is(err, ErrServiceAuthorityFetchMissing) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityFetchMissing, err)
	}
}

func TestCachedServiceAuthorityProvider_CachesUntilRefreshWindow(t *testing.T) {
	fetchCount := 0
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RefreshBefore: time.Minute,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			fetchCount++
			return &ServiceAuthorityToken{
				Token:     "service-token-" + string(rune('0'+fetchCount)),
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	first, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("first service authority failed: %v", err)
	}
	second, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("second service authority failed: %v", err)
	}
	if first != "service-token-1" || second != first || fetchCount != 1 {
		t.Fatalf("expected cached token, first=%q second=%q fetch=%d", first, second, fetchCount)
	}

	provider.expiresAt = time.Now().Add(30 * time.Second)
	third, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("third service authority failed: %v", err)
	}
	if third != "service-token-2" || fetchCount != 2 {
		t.Fatalf("expected refresh inside window, token=%q fetch=%d", third, fetchCount)
	}
}

func TestCachedServiceAuthorityProvider_RejectsEmptyToken(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return &ServiceAuthorityToken{}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	_, err = provider.ServiceAuthority(context.Background())
	if !errors.Is(err, ErrServiceAuthorityTokenMissing) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityTokenMissing, err)
	}
}

func TestNewServiceAuthorityToken_ParsesExpiredValue(t *testing.T) {
	expiresAt := time.Date(2026, 5, 31, 10, 30, 0, 0, time.UTC)
	token, err := NewServiceAuthorityToken(" service-token ", expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("new token failed: %v", err)
	}
	if token.Token != "service-token" || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestParseServiceAuthorityExpiresAt_AllowsPermanentValue(t *testing.T) {
	expiresAt, err := ParseServiceAuthorityExpiresAt(PermanentlyValidExpiredValue)
	if err != nil {
		t.Fatalf("parse permanent expired failed: %v", err)
	}
	if !expiresAt.IsZero() {
		t.Fatalf("expected zero time for permanent token, got %s", expiresAt)
	}
}

func TestValidateServiceAuthorityToken_RejectsExpiredToken(t *testing.T) {
	err := validateServiceAuthorityToken(&ServiceAuthorityToken{
		Token:     "service-token",
		ExpiresAt: time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC),
	}, time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrServiceAuthorityTokenExpired) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityTokenExpired, err)
	}
}
