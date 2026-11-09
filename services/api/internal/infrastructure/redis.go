package infrastructure

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct{ Client *redis.Client }

var limitScript = redis.NewScript(`local count=redis.call('INCR',KEYS[1]); if count==1 then redis.call('EXPIRE',KEYS[1],60) end; return count`)

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	count, err := limitScript.Run(ctx, r.Client, []string{"rate:" + key}).Int()
	return count <= limit, err
}
