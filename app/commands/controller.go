// Package commands contains the CLI commands for the application
package commands

import (
	"sync"
	"time"

	"github.com/hay-kot/ghdb/app/clients"
	"github.com/hay-kot/ghdb/app/commands/gdb"
	"github.com/urfave/cli/v2"
)

type Controller struct {
	LogLevel string
	CacheDir string

	GitDB       *gdb.GitDatabase
	GitHub      *clients.GitHub
	GitHubToken string
}

func (c *Controller) Sync(ctx *cli.Context) error {
	wg := sync.WaitGroup{}

	wg.Add(2)

	// Sync Repositories
	var repos []clients.Repository

	go func() {
		defer wg.Done()

    for _, user := range c.GitDB.Conf.Users {
      baseURL := "https://api.github.com"
      if (user.URL != "") {
        baseURL = user.URL
      }

      newRepos, err := c.GitHub.AllRepositoriesFor(baseURL, user.Name, !user.IsOrg, user.Token)
      if err != nil {
        panic(err)
      }

      repos = append(repos, newRepos...)
    }
	}()

	// Sync Pull Requests
	var prs []clients.PullRequest

	go func() {
		defer wg.Done()

    for _, user := range c.GitDB.Conf.Users {
      if (user.IsOrg) {
        continue
      }

      baseURL := "https://api.github.com"
      if (user.URL != "") {
        baseURL = user.URL
      }

      newPrs, err := c.GitHub.AllPullRequestsFor(baseURL, user.Name, user.Token)
      if err != nil {
        panic(err)
      }

      prs = append(prs, newPrs...)
    }
	}()

	wg.Wait()

	cache := gdb.Cache{
		Timestamp:    time.Now(),
		Repositories: repos,
		PullRequests: prs,
	}

	return c.GitDB.SetCache(cache)
}
