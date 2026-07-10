package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

const (
	searchIssuesPath  = "/search/issues"
	maxCommitsPerRepo = 50
	maxPRsPerRepo     = 20
)

// Provider returns the activity-evidence provider id.
func (p *Provider) Provider() string {
	return providerGitHub
}

// FetchEvidence loads commits and merged PRs for selected repos in window.
// Per-repo failures are skipped so one bad repo does not block the rest.
func (p *Provider) FetchEvidence(ctx context.Context, window service.TimeWindow) ([]service.ActivityEvidence, error) {
	if p.Queries == nil {
		return nil, nil
	}
	start := window.Start.UTC()
	end := window.End.UTC()
	if !end.After(start) {
		return nil, nil
	}

	repos, err := p.Queries.ListSelectedGitHubRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list selected github repos: %w", err)
	}
	if len(repos) == 0 {
		return nil, nil
	}

	out := make([]service.ActivityEvidence, 0)
	for _, repo := range repos {
		items := p.fetchRepoEvidence(ctx, repo, start, end)
		out = append(out, items...)
	}
	return out, nil
}

func (p *Provider) fetchRepoEvidence(ctx context.Context, repo sqlc.GithubRepo, start, end time.Time) []service.ActivityEvidence {
	accountID := strings.TrimSpace(repo.AccountID)
	fullName := strings.TrimSpace(repo.FullName)
	if accountID == "" || fullName == "" {
		return nil
	}

	out := make([]service.ActivityEvidence, 0)
	commits, err := p.fetchCommits(ctx, accountID, fullName, start, end)
	if err == nil {
		out = append(out, commits...)
	}
	prs, err := p.fetchMergedPRs(ctx, accountID, fullName, start, end)
	if err == nil {
		out = append(out, prs...)
	}
	return out
}

func (p *Provider) fetchCommits(ctx context.Context, accountID, fullName string, start, end time.Time) ([]service.ActivityEvidence, error) {
	path := "/repos/" + fullName + "/commits"
	out := make([]service.ActivityEvidence, 0, maxCommitsPerRepo)

	for page := 1; len(out) < maxCommitsPerRepo; page++ {
		q := url.Values{}
		q.Set("author", accountID)
		q.Set("since", start.Format(time.RFC3339))
		q.Set("until", end.Format(time.RFC3339))
		q.Set("per_page", strconv.Itoa(defaultPerPage))
		q.Set("page", strconv.Itoa(page))

		var items []commitItem
		if err := p.getJSON(ctx, accountID, path, q, &items); err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			if len(out) >= maxCommitsPerRepo {
				break
			}
			ev, ok := mapCommit(item, fullName, start, end)
			if !ok {
				continue
			}
			out = append(out, ev)
		}

		if len(items) < defaultPerPage {
			break
		}
	}
	return out, nil
}

func (p *Provider) fetchMergedPRs(ctx context.Context, accountID, fullName string, start, end time.Time) ([]service.ActivityEvidence, error) {
	// Search uses date-only merged: bounds; client-filter to half-open window.
	q := url.Values{}
	q.Set("q", fmt.Sprintf(
		"repo:%s is:pr is:merged author:%s merged:%s..%s",
		fullName,
		accountID,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	))
	q.Set("per_page", strconv.Itoa(maxPRsPerRepo))
	q.Set("sort", "updated")
	q.Set("order", "desc")

	var result searchIssuesResponse
	if err := p.getJSON(ctx, accountID, searchIssuesPath, q, &result); err != nil {
		return nil, err
	}

	out := make([]service.ActivityEvidence, 0, len(result.Items))
	for _, item := range result.Items {
		if len(out) >= maxPRsPerRepo {
			break
		}
		ev, ok := mapMergedPR(item, fullName, start, end)
		if !ok {
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

func mapCommit(item commitItem, fullName string, start, end time.Time) (service.ActivityEvidence, bool) {
	ts, ok := parseCommitTime(item)
	if !ok || !inWindow(ts, start, end) {
		return service.ActivityEvidence{}, false
	}

	sha := strings.TrimSpace(item.SHA)
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	message := strings.TrimSpace(item.Commit.Message)
	firstLine := firstLine(message)
	summary := firstLine
	if short != "" {
		if firstLine == "" {
			summary = short
		} else {
			summary = short + ": " + firstLine
		}
	}
	summary = prefixEvidenceSummary(fullName, summary)

	return service.ActivityEvidence{
		Provider: providerGitHub,
		Kind:     "commit",
		Start:    ts,
		End:      ts.Add(time.Second),
		Summary:  summary,
		Source:   fullName,
		Detail:   message,
		URL:      strings.TrimSpace(item.HTMLURL),
	}, true
}

func mapMergedPR(item searchIssueItem, fullName string, start, end time.Time) (service.ActivityEvidence, bool) {
	ts, ok := parsePRTime(item)
	if !ok || !inWindow(ts, start, end) {
		return service.ActivityEvidence{}, false
	}

	title := strings.TrimSpace(item.Title)
	summary := prefixEvidenceSummary(fullName, fmt.Sprintf("Merged PR #%d: %s", item.Number, title))
	detail := title
	body := strings.TrimSpace(item.Body)
	if body != "" {
		detail = title + "\n\n" + body
	}

	return service.ActivityEvidence{
		Provider: providerGitHub,
		Kind:     "pr",
		Start:    ts,
		End:      ts.Add(time.Second),
		Summary:  summary,
		Source:   fullName,
		Detail:   detail,
		URL:      strings.TrimSpace(item.HTMLURL),
	}, true
}

func parseCommitTime(item commitItem) (time.Time, bool) {
	raw := strings.TrimSpace(item.Commit.Author.Date)
	if raw == "" {
		raw = strings.TrimSpace(item.Commit.Committer.Date)
	}
	if raw == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

func parsePRTime(item searchIssueItem) (time.Time, bool) {
	raw := strings.TrimSpace(item.ClosedAt)
	if raw == "" && item.PullRequest != nil {
		raw = strings.TrimSpace(item.PullRequest.MergedAt)
	}
	if raw == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts.UTC(), true
}

func inWindow(ts, start, end time.Time) bool {
	return !ts.Before(start) && ts.Before(end)
}

func firstLine(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		return strings.TrimSpace(message[:i])
	}
	return message
}

func prefixEvidenceSummary(source, summary string) string {
	source = strings.TrimSpace(source)
	summary = strings.TrimSpace(summary)
	if source == "" {
		return summary
	}
	if summary == "" {
		return source
	}
	return source + " · " + summary
}

type commitItem struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type searchIssuesResponse struct {
	Items []searchIssueItem `json:"items"`
}

type searchIssueItem struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	ClosedAt    string `json:"closed_at"`
	PullRequest *struct {
		MergedAt string `json:"merged_at"`
	} `json:"pull_request"`
}
