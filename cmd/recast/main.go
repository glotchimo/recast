package main

import (
	"os"
	"os/signal"

	_ "net/http/pprof"

	"github.com/caarlos0/env/v11"
	"github.com/glotchimo/recast/internal/bot"
	"github.com/joho/godotenv"
)

var VERSION = "dev"

type Conf struct {
	Debug       bool   `env:"DEBUG"`
	Token       string `env:"BOT_TOKEN"`
	Intents     int    `env:"BOT_INTENTS" envDefault:"32509"`
	DatabaseURL string `env:"DATABASE_URL"`
	CacheURL    string `env:"REDIS_URL"`
	ShardID     int    `env:"SHARD_ID" envDefault:"0"`
	ShardCount  int    `env:"SHARD_COUNT" envDefault:"1"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	var conf Conf
	if err := env.Parse(&conf); err != nil {
		panic(err)
	}

	bot, err := bot.NewBot(conf.Debug, conf.DatabaseURL, conf.CacheURL, conf.Token, conf.ShardID, conf.ShardCount, conf.Intents)
	if err != nil {
		panic(err)
	}
	defer bot.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
}
