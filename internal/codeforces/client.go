package codeforces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://codeforces.com/api"

type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	lastCall   time.Time
}

type apiResponse[T any] struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
	Result  T      `json:"result"`
}

type Contest struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Phase            string `json:"phase"`
	DurationSeconds  int    `json:"durationSeconds"`
	StartTimeSeconds int64  `json:"startTimeSeconds"`
}

type Standings struct {
	Contest  Contest   `json:"contest"`
	Problems []Problem `json:"problems"`
}

type UserInfo struct {
	Handle     string `json:"handle"`
	Rating     int    `json:"rating"`
	MaxRating  int    `json:"maxRating"`
	TitlePhoto string `json:"titlePhoto"`
}

type Submission struct {
	ID                  int64   `json:"id"`
	ContestID           int     `json:"contestId"`
	CreationTimeSeconds int64   `json:"creationTimeSeconds"`
	RelativeTimeSeconds int64   `json:"relativeTimeSeconds"`
	Problem             Problem `json:"problem"`
	Author              Author  `json:"author"`
	Verdict             string  `json:"verdict"`
	PassedTestCount     int     `json:"passedTestCount"`
}

type Problem struct {
	ContestID int    `json:"contestId"`
	Index     string `json:"index"`
	Name      string `json:"name"`
}

type Author struct {
	ParticipantType  string `json:"participantType"`
	StartTimeSeconds int64  `json:"startTimeSeconds"`
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) GetContestList(ctx context.Context) ([]Contest, error) {
	return get[[]Contest](ctx, c, "/contest.list", map[string]string{"gym": "false"})
}

func (c *Client) GetUserStatus(ctx context.Context, handle string) ([]Submission, error) {
	return get[[]Submission](ctx, c, "/user.status", map[string]string{"handle": handle})
}

func (c *Client) GetUsersInfo(ctx context.Context, handles []string) ([]UserInfo, error) {
	return get[[]UserInfo](ctx, c, "/user.info", map[string]string{"handles": strings.Join(handles, ";")})
}

func (c *Client) GetContestStandings(ctx context.Context, contestID int) (Standings, error) {
	return get[Standings](ctx, c, "/contest.standings", map[string]string{
		"contestId": fmt.Sprintf("%d", contestID),
		"from":      "1",
		"count":     "1",
	})
}

func get[T any](ctx context.Context, c *Client, path string, query map[string]string) (T, error) {
	var zero T

	if err := c.waitRateLimit(ctx); err != nil {
		return zero, err
	}

	endpoint, err := url.Parse(baseURL + path)
	if err != nil {
		return zero, err
	}

	params := endpoint.Query()
	for key, value := range query {
		params.Set(key, value)
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return zero, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("codeforces %s returned %s", path, resp.Status)
	}

	var payload apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, err
	}
	if payload.Status != "OK" {
		return zero, fmt.Errorf("codeforces %s failed: %s", path, payload.Comment)
	}
	return payload.Result, nil
}

func (c *Client) waitRateLimit(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.lastCall.IsZero() {
		nextAllowed := c.lastCall.Add(2 * time.Second)
		if wait := time.Until(nextAllowed); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	c.lastCall = time.Now()
	return nil
}
