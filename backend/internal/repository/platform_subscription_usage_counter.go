package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type platformSubscriptionUsageCounter struct {
	rdb *redis.Client
}

func NewPlatformSubscriptionUsageCounter(rdb *redis.Client) service.PlatformSubscriptionUsageCounter {
	return &platformSubscriptionUsageCounter{rdb: rdb}
}

func platformSubscriptionUsedKey(userID int64, grantDate string) string {
	return fmt.Sprintf("platform_sub:used:%d:%s", userID, grantDate)
}

func (c *platformSubscriptionUsageCounter) GetPlatformSubscriptionUsed(ctx context.Context, userID int64, grantDate string) (float64, error) {
	if c == nil || c.rdb == nil || userID <= 0 || grantDate == "" {
		return 0, nil
	}
	value, err := c.rdb.Get(ctx, platformSubscriptionUsedKey(userID, grantDate)).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(value, 64)
}

func (c *platformSubscriptionUsageCounter) IncrPlatformSubscriptionUsed(ctx context.Context, userID int64, grantDate string, cost float64, ttl time.Duration) (float64, error) {
	if c == nil || c.rdb == nil || userID <= 0 || grantDate == "" || cost <= 0 {
		return 0, nil
	}
	key := platformSubscriptionUsedKey(userID, grantDate)
	pipe := c.rdb.TxPipeline()
	incr := pipe.IncrByFloat(ctx, key, cost)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

const ensurePlatformSubscriptionUsedAtLeastScript = `
local current = redis.call("GET", KEYS[1])
local desired = tonumber(ARGV[1])
if current == false or tonumber(current) < desired then
	redis.call("SET", KEYS[1], ARGV[1])
end
if tonumber(ARGV[2]) > 0 then
	redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 1
`

func (c *platformSubscriptionUsageCounter) EnsurePlatformSubscriptionUsedAtLeast(ctx context.Context, userID int64, grantDate string, used float64, ttl time.Duration) error {
	if c == nil || c.rdb == nil || userID <= 0 || grantDate == "" || used <= 0 {
		return nil
	}
	_, err := c.rdb.Eval(ctx, ensurePlatformSubscriptionUsedAtLeastScript,
		[]string{platformSubscriptionUsedKey(userID, grantDate)},
		strconv.FormatFloat(used, 'f', -1, 64),
		int(ttl.Seconds()),
	).Result()
	return err
}

func (c *platformSubscriptionUsageCounter) GetPlatformSubscriptionCache(ctx context.Context, key string) ([]byte, bool, error) {
	if c == nil || c.rdb == nil || key == "" {
		return nil, false, nil
	}
	value, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (c *platformSubscriptionUsageCounter) SetPlatformSubscriptionCache(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.rdb == nil || key == "" || len(value) == 0 {
		return nil
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}
