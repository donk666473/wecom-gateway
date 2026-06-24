// Package db 提供 Redis 客户端初始化和缓存操作。
// 参照钉钉对接项目中的 db/redis 和 redis_prefix_hook 模块。
// 支持 Key 前缀自动注入，实现多应用 Key 隔离。
package db

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisConfig Redis 连接配置
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	Prefix   string
}

var (
	// RDB 全局 Redis 客户端
	RDB *redis.Client
	// redisPrefix Redis Key 前缀
	redisPrefix string
)

func ensureRedis() error {
	if RDB == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return nil
}

// InitRedis 初始化 Redis 连接并注入前缀 Hook。
func InitRedis(cfg *RedisConfig) error {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	redisPrefix = cfg.Prefix

	RDB = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		MaxRetries:   3,
	})

	// 注入前缀 Hook
	RDB.AddHook(&redisPrefixHook{prefix: cfg.Prefix})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}

	return nil
}

// RedisGet 从 Redis 获取字符串值
func RedisGet(key string) (string, error) {
	if err := ensureRedis(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.Get(ctx, key).Result()
}

// RedisSet 设置 Redis 字符串值（带过期时间）
func RedisSet(key, value string, expiration time.Duration) error {
	if err := ensureRedis(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.Set(ctx, key, value, expiration).Err()
}

// RedisSetNX 设置 Redis 值（仅当 key 不存在时），返回 true 表示设置成功。
// 用于消息去重等场景。
func RedisSetNX(key, value string, expiration time.Duration) bool {
	if err := ensureRedis(); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.SetNX(ctx, key, value, expiration).Val()
}

// RedisDel 删除 Redis Key
func RedisDel(key string) error {
	if err := ensureRedis(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.Del(ctx, key).Err()
}

// RedisExists 检查 Key 是否存在
func RedisExists(key string) (bool, error) {
	if err := ensureRedis(); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	n, err := RDB.Exists(ctx, key).Result()
	return n > 0, err
}

// RedisGetDel 获取值并删除 key（用于一次性 Token 读取）
func RedisGetDel(key string) (string, error) {
	if err := ensureRedis(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.GetDel(ctx, key).Result()
}

// ============================================================================
// Redis Key 前缀自动注入 Hook — 参照钉钉对接项目最佳实践
// ============================================================================

// redisPrefixHook 实现 redis.Hook 接口，自动为所有 Key 添加统一前缀。
type redisPrefixHook struct {
	prefix string
}

func (h *redisPrefixHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	h.prefixCmd(cmd)
	return ctx, nil
}

func (h *redisPrefixHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	return nil
}

func (h *redisPrefixHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	for _, cmd := range cmds {
		h.prefixCmd(cmd)
	}
	return ctx, nil
}

func (h *redisPrefixHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	return nil
}

// prefixCmd 根据命令类型为 Key 添加前缀
func (h *redisPrefixHook) prefixCmd(cmd redis.Cmder) {
	if h.prefix == "" {
		return
	}
	cmdName := cmd.Name()
	args := cmd.Args()
	if len(args) < 2 {
		return
	}
	// 不同类型的命令 key 位置不同
	switch cmdName {
	case "GET", "SET", "DEL", "EXISTS", "EXPIRE", "TTL", "TYPE",
		"HGET", "HSET", "HDEL", "HGETALL", "HMSET", "HLEN",
		"LPUSH", "RPUSH", "LPOP", "RPOP", "LLEN", "LRANGE",
		"SADD", "SREM", "SMEMBERS", "SCARD", "SISMEMBER",
		"ZADD", "ZREM", "ZRANGE", "ZCARD", "ZSCORE",
		"SETNX", "SETEX", "GETSET", "GETDEL",
		"INCR", "DECR", "INCRBY", "DECRBY",
		"DUMP", "RESTORE", "PEXPIRE", "EXPIREAT":
		if key, ok := args[1].(string); ok && !hasPrefix(key, h.prefix) {
			args[1] = h.prefix + ":" + key
		}
	}
}

// hasPrefix 检查 key 是否已经包含前缀（防重复添加）
func hasPrefix(key, prefix string) bool {
	if prefix == "" {
		return false
	}
	prefixWithSep := prefix + ":"
	return len(key) >= len(prefixWithSep) && key[:len(prefixWithSep)] == prefixWithSep
}
