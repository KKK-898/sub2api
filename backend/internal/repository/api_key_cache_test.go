//go:build unit

package repository

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestApiKeyRateLimitKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		expected string
	}{
		{
			name:     "normal_user_id",
			userID:   123,
			expected: "apikey:ratelimit:123",
		},
		{
			name:     "zero_user_id",
			userID:   0,
			expected: "apikey:ratelimit:0",
		},
		{
			name:     "negative_user_id",
			userID:   -1,
			expected: "apikey:ratelimit:-1",
		},
		{
			name:     "max_int64",
			userID:   math.MaxInt64,
			expected: "apikey:ratelimit:9223372036854775807",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := apiKeyRateLimitKey(tc.userID)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestAPIKeyCache_DeleteAuthCachesAndPublish(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &apiKeyCache{rdb: rdb}

	for _, cacheKey := range []string{"hash-1", "hash-2"} {
		require.NoError(t, rdb.Set(ctx, apiKeyAuthCacheKey(cacheKey), "snapshot", time.Minute).Err())
	}
	subscriber := rdb.Subscribe(ctx, authCacheInvalidateChannel)
	_, err := subscriber.Receive(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = subscriber.Close() })

	require.NoError(t, cache.DeleteAuthCachesAndPublish(ctx, []string{"hash-1", "", "hash-2"}))
	require.False(t, mr.Exists(apiKeyAuthCacheKey("hash-1")))
	require.False(t, mr.Exists(apiKeyAuthCacheKey("hash-2")))

	received := make([]string, 0, 2)
	timeout := time.After(time.Second)
	for len(received) < 2 {
		select {
		case message := <-subscriber.Channel():
			received = append(received, message.Payload)
		case <-timeout:
			t.Fatal("timed out waiting for batched auth cache invalidations")
		}
	}
	require.ElementsMatch(t, []string{"hash-1", "hash-2"}, received)
}
