// Package clients implements Git Clients used for API interactions
package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/hay-kot/ghdb/app/clients/httpclient"
)

type KeyPair = map[string]string

type GitHub struct {
	http *httpclient.Client

	// URLLookup is a map of Github User/Org names and their respective URLs
	URLLookup KeyPair
}

func NewGitHub() *GitHub {
	client := httpclient.New(http.DefaultClient, "")

	client.Use(
		httpclient.MwContentType("application/json"),
	)

	return &GitHub{http: client}
}

type Owner struct {
	Login string `json:"login"`
}

type Repository struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Owner       Owner  `json:"owner"`
	CloneURL    string `json:"clone_url"`
	WebURL      string `json:"html_url"`
	Description string `json:"description"`
}

func (gh *GitHub) AllRepositoriesFor(baseURL, namespace string, user bool, token string) ([]Repository, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	var (
		pageSize   = 100
		page       = 1
		resultsLen = -1
	)

	// Construct URL

	repositories := make([]Repository, 0)

	pathPrefix := "users"
	if !user {
		pathPrefix = "orgs"
	}

	for resultsLen == -1 || resultsLen == pageSize {
		resp, err := gh.http.Get(
			gh.http.Pathf("%s/%s/%s/repos?per_page=%d&page=%d", baseURL, pathPrefix, namespace, pageSize, page),
			httpclient.MwBearerToken(token),
		)
		if err != nil {
			return nil, err
		}

		defer func() { _ = resp.Body.Close() }()

		var repos []Repository

		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			return nil, err
		}

		repositories = append(repositories, repos...)

		resultsLen = len(repos)
		page++
	}

	return repositories, nil
}

type PullRequest struct {
	Number        int    `json:"number"`
	Title         string `json:"title"`
	User          Owner  `json:"user"`
	URL           string `json:"html_url"`
	Draft         bool   `json:"draft"`
	RepositoryURL string `json:"repository_url"`
}

func (pr PullRequest) RepositoryName() string {
	// Parse the repository name from the URL
	// Assume last two segments are the owner and repo name

	name := pr.RepositoryURL
	name = strings.TrimSuffix(name, "/")
	segments := strings.Split(name, "/")
	if len(segments) < 2 {
		return pr.RepositoryURL
	}

	return segments[len(segments)-2] + "/" + segments[len(segments)-1]
}

type searchResults struct {
	Items []PullRequest `json:"items"`
}

// AllPullRequestsFor  all repositories for a given user
// Orgs are not supported for this method
func (gh *GitHub) AllPullRequestsFor(baseURL, user string, token string) ([]PullRequest, error) {
	if user == "" {
		return nil, fmt.Errorf("user is required")
	}

	var (
		pageSize   = 100
		page       = 1
		resultsLen = -1
	)

	var prs []PullRequest

	for resultsLen == -1 || resultsLen == pageSize {
		resp, err := gh.http.Get(
			gh.http.Pathf("%s/search/issues?q=state:open+type:pr+author:%s&per_page=%d&page=%d", baseURL, user, pageSize, page),
			httpclient.MwBearerToken(token),
		)
		if err != nil {
			return nil, err
		}

		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			io.Copy(os.Stderr, resp.Body)
			return nil, fmt.Errorf("failed to fetch pull requests for user %s", user)
		}

		var results searchResults

		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return nil, err
		}

		prs = append(prs, results.Items...)

		resultsLen = len(results.Items)
		page++
	}

	return prs, nil
}
