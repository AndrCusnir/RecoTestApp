package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const asanaBaseURL = "https://app.asana.com/api/1.0"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	PAT        string
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(pat string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		PAT:        pat,
		HTTPClient: httpClient,
		BaseURL:    asanaBaseURL,
		MaxRetries: 3,
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) doRequest(ctx context.Context, target string) (*http.Response, error) {
	for retries := 0; ; retries++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+c.PAT)

		response, err := c.HTTPClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		if response.StatusCode != http.StatusTooManyRequests {
			return response, nil
		}

		retryAfter := strings.TrimSpace(response.Header.Get("Retry-After"))
		response.Body.Close()
		seconds, err := strconv.Atoi(retryAfter)
		if err != nil || seconds < 0 {
			return nil, fmt.Errorf("invalid Retry-After header %q", retryAfter)
		}
		if retries >= c.MaxRetries {
			return nil, fmt.Errorf("request rate limited after %d retries", retries)
		}

		sleep := c.Sleep
		if sleep == nil {
			sleep = sleepContext
		}
		if err := sleep(ctx, time.Duration(seconds)*time.Second); err != nil {
			return nil, fmt.Errorf("wait after rate limit: %w", err)
		}
	}
}

type usersPage struct {
	Data     []json.RawMessage `json:"data"`
	NextPage *nextPage         `json:"next_page"`
}

type nextPage struct {
	Offset string `json:"offset"`
	Path   string `json:"path"`
	URI    string `json:"uri"`
}

type entityGID struct {
	GID string `json:"gid"`
}

func (c *Client) ExtractUsers(
	ctx context.Context,
	workspaceGID string,
	consume func(gid string, entity json.RawMessage) error,
) error {
	requestURL, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + "/users")
	if err != nil {
		return fmt.Errorf("build users URL: %w", err)
	}
	query := requestURL.Query()
	query.Set("workspace", workspaceGID)
	query.Set("limit", "100")

	for {
		requestURL.RawQuery = query.Encode()
		response, err := c.doRequest(ctx, requestURL.String())
		if err != nil {
			return err
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return fmt.Errorf("users request returned %s", response.Status)
		}

		var page usersPage
		decodeErr := json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode users response: %w", decodeErr)
		}

		for _, entity := range page.Data {
			var view entityGID
			if err := json.Unmarshal(entity, &view); err != nil {
				return fmt.Errorf("decode user GID: %w", err)
			}
			if view.GID == "" {
				return fmt.Errorf("user is missing gid")
			}
			if err := consume(view.GID, entity); err != nil {
				return fmt.Errorf("consume user %q: %w", view.GID, err)
			}
		}

		if page.NextPage == nil {
			return nil
		}
		if page.NextPage.Offset == "" {
			return fmt.Errorf("users response contains next_page without offset")
		}
		query.Set("offset", page.NextPage.Offset)
	}
}

type projectsPage struct {
	Data     []json.RawMessage `json:"data"`
	NextPage *nextPage         `json:"next_page"`
}

func (c *Client) ExtractProjects(
	ctx context.Context,
	workspaceGID string,
	consume func(gid string, entity json.RawMessage) error,
) error {
	requestURL, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + "/projects")
	if err != nil {
		return fmt.Errorf("build projects URL: %w", err)
	}
	query := requestURL.Query()
	query.Set("workspace", workspaceGID)
	query.Set("limit", "100")

	for {
		requestURL.RawQuery = query.Encode()
		response, err := c.doRequest(ctx, requestURL.String())
		if err != nil {
			return err
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return fmt.Errorf("projects request returned %s", response.Status)
		}

		var page projectsPage
		decodeErr := json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode projects response: %w", decodeErr)
		}

		for _, entity := range page.Data {
			var view entityGID
			if err := json.Unmarshal(entity, &view); err != nil {
				return fmt.Errorf("decode project GID: %w", err)
			}
			if view.GID == "" {
				return fmt.Errorf("project is missing gid")
			}
			if err := consume(view.GID, entity); err != nil {
				return fmt.Errorf("consume project %q: %w", view.GID, err)
			}
		}

		if page.NextPage == nil {
			return nil
		}
		if page.NextPage.Offset == "" {
			return fmt.Errorf("projects response contains next_page without offset")
		}
		query.Set("offset", page.NextPage.Offset)
	}
}
