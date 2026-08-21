package env

import "github.com/caarlos0/env/v11"

type Config struct {
	ServerPort        string `env:"SERVER_PORT" envDefault:"30666"`
	LogFormat         string `env:"LOG_FORMAT" envDefault:"text"`
	DBPath            string `env:"DB_PATH" envDefault:"db/filedb.sqlite"`
	LitestreamReplica string `env:"LITESTREAM_REPLICA" envDefault:""`
}

var cfg Config

func Load() Config {
	cfg = Config{}
	if err := env.Parse(&cfg); err != nil {
		panic(err) // this one is OK
	}
	return cfg
}

func Get() Config {
	return cfg
}
