package bitbucket

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dylanbr0wn/shiet/internal/db/sqlc"
	"github.com/dylanbr0wn/shiet/internal/service"
)

const (
	commitsPath       = "/repositories"
	maxCommitsPerRepo = 50
	maxCommitPages    = 10
)

// Provider returns the activity-evidence provider id.
func (p *Provider) Provider() string {
	return providerBitbucket
}

// FetchEvidence loads commits for selected repos in window.
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

	repos, err := p.Queries.ListSelectedBitbucketRepos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list selected bitbucket repos: %w", err)
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

func (p *Provider) fetchRepoEvidence(ctx context.Context, repo sqlc.BitbucketRepo, start, end time.Time) []service.ActivityEvidence {
	accountID := strings.TrimSpace(repo.AccountID)
	fullName := strings.TrimSpace(repo.FullName)
	if accountID == "" || fullName == "" {
		return nil
	}

	commits, err := p.fetchCommits(ctx, accountID, fullName, start, end)
	if err != nil {
		return nil
	}
	return commits
}

func (p *Provider) fetchCommits(ctx context.Context, accountID, fullName string, start, end time.Time) ([]service.ActivityEvidence, error) {
	workspace, repoSlug, ok := splitRepoFullName(fullName)
	if !ok {
		return nil, fmt.Errorf("invalid bitbucket repo full_name %q", fullName)
	}

	out := make([]service.ActivityEvidence, 0, maxCommitsPerRepo)
	nextURL := fmt.Sprintf(
		"%s%s/%s/%s/commits?pagelen=%d",
		p.baseURL(),
		commitsPath,
		workspace,
		repoSlug,
		defaultPageSize,
	)

	for page := 0; page < maxCommitPages && nextURL != "" && len(out) < maxCommitsPerRepo; page++ {
		var pageResp paginatedResponse[commitItem]
		if err := p.getAbsoluteJSON(ctx, accountID, nextURL, &pageResp); err != nil {
			return nil, err
		}
		if len(pageResp.Values) == 0 {
			break
		}

		stopPaging := false
		for _, item := range pageResp.Values {
			if len(out) >= maxCommitsPerRepo {
				break
			}

			ts, ok := parseCommitTime(item)
			if !ok {
				continue
			}
			if ts.Before(start) {
				stopPaging = true
				break
			}
			if !ts.Before(end) {
				continue
			}
			if !commitAuthorMatches(item, accountID) {
				continue
			}

			ev, ok := mapCommit(item, fullName, ts)
			if !ok {
				continue
			}
			out = append(out, ev)
		}

		if stopPaging {
			break
		}
		nextURL = strings.TrimSpace(pageResp.Next)
	}
	return out, nil
}

func splitRepoFullName(fullName string) (workspace, repoSlug string, ok bool) {
	parts := strings.Split(strings.TrimSpace(fullName), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	workspace = strings.TrimSpace(parts[0])
	repoSlug = strings.TrimSpace(parts[1])
	if workspace == "" || repoSlug == "" {
		return "", "", false
	}
	return workspace, repoSlug, true
}

func commitAuthorMatches(item commitItem, accountID string) bool {
	authorUUID := strings.TrimSpace(item.Author.User.UUID)
	if authorUUID == "" {
		return false
	}
	return strings.EqualFold(authorUUID, strings.TrimSpace(accountID))
}

func mapCommit(item commitItem, fullName string, ts time.Time) (service.ActivityEvidence, bool) {
	hash := strings.TrimSpace(item.Hash)
	short := hash
	if len(short) > 7 {
		short = short[:7]
	}
	message := strings.TrimSpace(item.Message)
	first := firstLine(message)
	summary := first
	if short != "" {
		if first == "" {
			summary = short
		} else {
			summary = short + ": " + first
		}
	}
	summary = prefixEvidenceSummary(fullName, summary)

	url := ""
	if item.Links.HTML != nil {
		url = strings.TrimSpace(item.Links.HTML.Href)
	}

	return service.ActivityEvidence{
		Provider: providerBitbucket,
		Kind:     "commit",
		Start:    ts,
		End:      ts.Add(time.Second),
		Summary:  summary,
		Source:   fullName,
		Detail:   message,
		URL:      url,
	}, true
}

func parseCommitTime(item commitItem) (time.Time, bool) {
	raw := strings.TrimSpace(item.Date)
	if raw == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts.UTC(), true
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
	Hash    string `json:"hash"`
	Date    string `json:"date"`
	Message string `json:"message"`
	Author  struct {
		User struct {
			UUID string `json:"uuid"`
		} `json:"user"`
	} `json:"author"`
	Links struct {
		HTML *struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}
