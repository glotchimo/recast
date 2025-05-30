package cache

import (
	"log/slog"

	dg "github.com/bwmarrin/discordgo"
	"github.com/glotchimo/recast/internal/database"
	"github.com/graxinc/errutil"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	s *dg.Session
	c *redis.Client
	l *slog.Logger
	d *database.Database
}

func NewCache(url string, s *dg.Session, l *slog.Logger, d *database.Database) (*Cache, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, errutil.With(err)
	}
	c := redis.NewClient(opt)

	return &Cache{s, c, l, d}, nil
}

func (c *Cache) Close() error {
	return c.c.Close()
}
