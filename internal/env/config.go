package env

import "github.com/caarlos0/env/v11"

type Config struct {
	ServerPort         string `env:"SERVER_PORT" envDefault:"30666"`
	LogFormat          string `env:"LOG_FORMAT" envDefault:"text"`
	DBPath             string `env:"DB_PATH" envDefault:"db/filedb.sqlite"`
	ReplicaUrl         string `env:"LITESTREAM_REPLICA_URL" envDefault:""`
	LitestreamMetaPath string `env:"LITESTREAM_META_PATH" envDefault:"./litestream-cache"`
	ReplicationEnabled bool   `env:"REPLICATION_ENABLED" envDefault:"false"`
	SiteFilePath       string `env:"SITE_FILE_PATH" envDefault:"priv/site.yml"`
	PprofEnabled       bool   `env:"PPROF_ENABLED" envDefault:"false"`
	AltchaSecret       string `env:"ALTCHA_SECRET" envDefault:""`
	AltchaCost         int    `env:"ALTCHA_COST" envDefault:"5000"`
	AltchaExpiryMin    int    `env:"ALTCHA_EXPIRY_MINUTES" envDefault:"10"`
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
