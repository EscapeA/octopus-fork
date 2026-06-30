package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/lingyuins/octopus/internal/utils/log"
	"github.com/redis/go-redis/v9"
)

// redisChannelDelay 是 ChannelDelayStore 的 Redis 实现。
// key 形如 octopus:delay:{channelID}（Hash），field=url，value=延迟毫秒。
// 现状探测结果仅存内存 chCache，重启丢失；Redis 实现持久化，重启后保留。
type redisChannelDelay struct {
	c *redis.Client
}

func newRedisChannelDelay(c *redis.Client) *redisChannelDelay { return &redisChannelDelay{c: c} }

func (r *redisChannelDelay) SetDelay(ctx context.Context, channelID int, url string, delay int) error {
	key := withPrefix(fmt.Sprintf("delay:%d", channelID))
	if err := r.c.HSet(ctx, key, url, delay).Err(); err != nil {
		return err
	}
	// 探测结果无严格过期需求，但加一个较长 TTL 避免废弃 channel 残留无界增长。
	// 7 天后若该 channel 未再被探测则自动清理（下次探测会重新写入并续期）。
	r.c.Expire(ctx, key, 7*24*time.Hour)
	return nil
}

func (r *redisChannelDelay) GetDelays(ctx context.Context, channelID int) (map[string]int, error) {
	key := withPrefix(fmt.Sprintf("delay:%d", channelID))
	raw, err := r.c.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(raw))
	for url, val := range raw {
		delay, parseErr := strconv.Atoi(val)
		if parseErr != nil {
			continue
		}
		result[url] = delay
	}
	return result, nil
}

func (r *redisChannelDelay) DeleteChannel(ctx context.Context, channelID int) error {
	key := withPrefix(fmt.Sprintf("delay:%d", channelID))
	if err := r.c.Del(ctx, key).Err(); err != nil {
		log.Warnf("redis delete channel delay %d: %v", channelID, err)
		return err
	}
	return nil
}
