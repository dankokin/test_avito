package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

func (cfg *Config) LoadFromYaml(file string) {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		panic(err)
	}
}
