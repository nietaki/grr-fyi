package site

import "github.com/knadh/koanf/v2"
import "github.com/knadh/koanf/providers/file"
import "github.com/knadh/koanf/parsers/yaml"

// read yml
//

type SiteConfig struct {
	k *koanf.Koanf
}

func (conf SiteConfig) Get(key string) string {
	return conf.k.String(key)
}

func Read() SiteConfig {
	k := koanf.New(".")
	f := file.Provider("priv/site.yml")
	if err := k.Load(f, yaml.Parser()); err != nil {
		panic("could not load priv/site.yml")
	}

	return SiteConfig{k: k}
}
