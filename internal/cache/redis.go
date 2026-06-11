package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/model"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/shortener"
)

const (
	linkTTL      = 10 * time.Minute
	tokenBucketL = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local refill_rate = limit / window
local values = redis.call("HMGET", key, "tokens", "updated_at")
local tokens = tonumber(values[1]) or limit
local updated_at = tonumber(values[2]) or now
local elapsed = math.max(0, now - updated_at)
tokens = math.min(limit, tokens + (elapsed * refill_rate))
local allowed = 0
if tokens >= 1 then
  allowed = 1
  tokens = tokens - 1
end
redis.call("HMSET", key, "tokens", tostring(tokens), "updated_at", tostring(now))
redis.call("EXPIRE", key, math.ceil(window * 2))
return {allowed, tostring(tokens)}
`
)

var ErrCacheMiss = errors.New("cache miss")

type RedisCache struct {
	client *redis.Client
}

func NewRedis(ctx context.Context, cfg config.Config) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) GetLink(ctx context.Context, code, host string) (model.Link, error) {
	host = shortener.NormalizeDomain(host)
	keys := []string{linkKey("", code)}
	if host != "" {
		keys = append([]string{linkKey(host, code)}, keys...)
	}

	for _, key := range keys {
		raw, err := c.client.Get(ctx, key).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return model.Link{}, err
		}

		var link model.Link
		if err := json.Unmarshal(raw, &link); err != nil {
			_ = c.client.Del(ctx, key).Err()
			return model.Link{}, ErrCacheMiss
		}
		if link.CustomDomain != "" && host != "" && link.CustomDomain != host {
			continue
		}
		if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
			_ = c.client.Del(ctx, key).Err()
			return model.Link{}, ErrCacheMiss
		}
		return link, nil
	}

	return model.Link{}, ErrCacheMiss
}

func (c *RedisCache) SetLink(ctx context.Context, link model.Link) error {
	raw, err := json.Marshal(link)
	if err != nil {
		return err
	}

	ttl := linkTTL
	if link.ExpiresAt != nil {
		untilExpiry := time.Until(*link.ExpiresAt)
		if untilExpiry <= 0 {
			return nil
		}
		if untilExpiry < ttl {
			ttl = untilExpiry
		}
	}

	pipe := c.client.Pipeline()
	pipe.Set(ctx, linkKey("", link.Code), raw, ttl)
	if link.CustomDomain != "" {
		pipe.Set(ctx, linkKey(link.CustomDomain, link.Code), raw, ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *RedisCache) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	result, err := c.client.Eval(ctx, tokenBucketL, []string{"rate:" + key}, limit, int(window.Seconds()), time.Now().Unix()).Result()
	if err != nil {
		return false, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) == 0 {
		return false, fmt.Errorf("unexpected redis rate-limit response: %T", result)
	}

	switch allowed := values[0].(type) {
	case int64:
		return allowed == 1, nil
	case string:
		return allowed == "1", nil
	case []byte:
		return string(allowed) == "1", nil
	default:
		parsed, err := strconv.Atoi(fmt.Sprint(allowed))
		return parsed == 1, err
	}
}

func linkKey(domain, code string) string {
	code = strings.TrimSpace(code)
	if domain == "" {
		return "link::" + code
	}
	return "link:" + shortener.NormalizeDomain(domain) + ":" + code
}
