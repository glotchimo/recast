package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	dg "github.com/bwmarrin/discordgo"
	"github.com/glotchimo/recast/internal/database"
	"github.com/graxinc/errutil"
	"github.com/redis/go-redis/v9"
)

const (
	defaultExpiration = 168 * time.Hour
	fallbackMaxSize   = 10000
	cbThreshold       = 5
	cbResetTimeout    = 30 * time.Second
)

type Cache struct {
	s        *dg.Session
	c        *redis.Client
	l        *slog.Logger
	d        *database.Database
	cb       *CircuitBreaker
	fallback *FallbackCache
}

func NewCache(url string, s *dg.Session, l *slog.Logger, d *database.Database) (*Cache, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, errutil.With(err)
	}
	c := redis.NewClient(opt)

	return &Cache{
		s:        s,
		c:        c,
		l:        l,
		d:        d,
		cb:       NewCircuitBreaker(cbThreshold, cbResetTimeout),
		fallback: NewFallbackCache(fallbackMaxSize),
	}, nil
}

func (c *Cache) Close() error {
	return c.c.Close()
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if !c.cb.Allow() {
		if data, ok := c.fallback.Get(key); ok {
			return data, nil
		}
		return nil, fmt.Errorf("circuit breaker open and key not in fallback")
	}

	data, err := c.c.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			c.cb.RecordFailure()
			if c.cb.IsOpen() {
				c.l.Warn("redis circuit breaker opened")
			}
		}
		if data, ok := c.fallback.Get(key); ok {
			return data, nil
		}
		return nil, err
	}

	c.cb.RecordSuccess()
	c.fallback.Set(key, data, defaultExpiration)
	return data, nil
}

func (c *Cache) Set(ctx context.Context, key string, data []byte, expiration time.Duration) error {
	if expiration == 0 {
		expiration = defaultExpiration
	}

	c.fallback.Set(key, data, expiration)

	if !c.cb.Allow() {
		return nil
	}

	if err := c.c.Set(ctx, key, data, expiration).Err(); err != nil {
		c.cb.RecordFailure()
		if c.cb.IsOpen() {
			c.l.Warn("redis circuit breaker opened")
		}
		return errutil.With(err)
	}

	c.cb.RecordSuccess()
	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	c.fallback.Delete(key)

	if !c.cb.Allow() {
		return nil
	}

	if err := c.c.Del(ctx, key).Err(); err != nil {
		c.cb.RecordFailure()
		return errutil.With(err)
	}

	c.cb.RecordSuccess()
	return nil
}

func (c *Cache) Client() *redis.Client {
	return c.c
}
