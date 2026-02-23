package env

import "github.com/caarlos0/env/v11"

type Config struct {
	ServerPort string `env:"SERVER_PORT" envDefault:"50666"`
}

func Load() Config {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}
	return cfg
}
