// Package gdb is the core of GitDatabase
package gdb

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hay-kot/ghdb/app/clients"
	"gopkg.in/yaml.v3"
)

type UserOrOrg struct {
	Name  string `yaml:"name"`   // the name of the user or org
	Token string `yaml:"token"`  // the token to use for the user or org
	IsOrg bool   `yaml:"is_org"` // if true, the name is an org, otherwise it's a user

	// URL is an optional base URL to use for the user or org
	// useful for GitHub Enterprise hosted on-prem
	URL string `yaml:"url"`
}

type Config struct {
	Users []UserOrOrg `yaml:"users"`
}

func ReadConfig(reader io.Reader) (*Config, error) {
	conf := &Config{}
	return conf, yaml.NewDecoder(reader).Decode(conf)
}

type GitDatabase struct {
	Conf      *Config
	CacheFile string
	cache     *Cache
}

func New(cacheFile string, conf *Config) *GitDatabase {
	return &GitDatabase{
		Conf:      conf,
		CacheFile: cacheFile,
	}
}

func (g *GitDatabase) LoadCache() (Cache, error) {
	if g.cache == nil {
		f, err := os.Open(g.CacheFile)
		if err != nil {
			return Cache{}, fmt.Errorf("failed to open cache file (%s): %w", g.CacheFile, err)
		}

		defer func() { _ = f.Close() }()
		err = json.NewDecoder(f).Decode(&g.cache)
		if err != nil {
			return Cache{}, fmt.Errorf("failed to decode cache file (%s): %w", g.CacheFile, err)
		}
	}

	return *g.cache, nil
}

func (g *GitDatabase) SetCache(c Cache) error {
	f, err := os.Create(g.CacheFile)
	if err != nil {
		return fmt.Errorf("failed to create cache file (%s): %w", g.CacheFile, err)
	}

	defer func() { _ = f.Close() }()
	return json.NewEncoder(f).Encode(c)
}

type Cache struct {
	Timestamp    time.Time             `json:"timestamp"`
	Repositories []clients.Repository  `json:"repositories"`
	PullRequests []clients.PullRequest `json:"pull_requests"`
}
