package config

import (
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	Port int `yaml:"port"`

	AllowedDomains AllowedDomainsMapping `yaml:"allowed_domains"`

	ResolveTimeout time.Duration `yaml:"resolve_timeout"`
	CacheDuration  time.Duration `yaml:"cache_duration"`

	DuckDNS struct {
		Domain string `yaml:"domain"`
		Token  string `yaml:"token"`
	} `yaml:"duckdns"`

	EMail string `yaml:"email"`
}

type AllowedDomainsMapping map[string]bool

func (a AllowedDomainsMapping) UnmarshalYAML(value *yaml.Node) (err error) {
	var list []string

	err = value.Decode(&list)
	if err != nil {
		return
	}

	for _, domain := range list {
		a[normalizeDomain(domain)] = true
	}

	return
}

func (a AllowedDomainsMapping) Contains(domain string) bool {
	return a[normalizeDomain(domain)]
}

func normalizeDomain(s string) string {
	return strings.TrimPrefix(s, "www.")
}

func Parse(filename string) (c Settings, err error) {
	c.AllowedDomains = make(AllowedDomainsMapping)

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(&c)
	if err != nil {
		return
	}

	return
}
