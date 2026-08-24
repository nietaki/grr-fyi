package site

import (
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/nietaki/grr-fyi/internal/env"
)

type SiteConfig struct {
	k *koanf.Koanf
}

func (conf SiteConfig) Get(key string) string {
	return conf.k.String(key)
}

func Read(cfg env.Config) SiteConfig {
	k := koanf.New(".")
	f := file.Provider(cfg.SiteFilePath)
	if err := k.Load(f, yaml.Parser()); err != nil {
		panic("could not load site config: " + cfg.SiteFilePath)
	}

	return SiteConfig{k: k}
}
