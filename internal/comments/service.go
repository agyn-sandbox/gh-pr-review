package comments

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Agyn-sandbox/gh-pr-review/internal/ghcli"
	"github.com/Agyn-sandbox/gh-pr-review/internal/resolver"
	review "github.com/Agyn-sandbox/gh-pr-review/internal/review"
)

const autoSubmitSummary = "Auto-submitting pending review to unblock threaded reply via gh-pr-review."

// Service provides high-level review comment operations.
type Service struct {
	API ghcli.API
}

// ReplyOptions contains the payload for replying to a review comment.
type ReplyOptions struct {
	CommentID int64
	Body      string
}

// NewService constructs a Service using the provided API client.
func NewService(api ghcli.API) *Service {
	return &Service{API: api}
}

// Reply posts a reply to an existing review comment, automatically submitting any pending reviews owned by the user when necessary.
func (s *Service) Reply(pr resolver.Identity, opts ReplyOptions) (json.RawMessage, error) {
	if opts.CommentID <= 0 {
		return nil, errors.New("invalid comment id")
	}
	if strings.TrimSpace(opts.Body) == "" {
		return nil, errors.New("reply body is required")
	}

	payload := map[string]interface{}{
		"body": opts.Body,
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments/%d/replies", pr.Owner, pr.Repo, pr.Number, opts.CommentID)

	var reply json.RawMessage
	err := s.API.REST("POST", path, nil, payload, &reply)
	if err == nil {
		return reply, nil
	}

	apiErr := &ghcli.APIError{}
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 422 || !apiErr.ContainsLower("pending review") {
		return nil, err
	}

	if err := s.autoSubmitPendingReviews(pr); err != nil {
		return nil, fmt.Errorf("failed to submit pending review: %w", err)
	}

	if err := s.API.REST("POST", path, nil, payload, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

func (s *Service) currentLogin() (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := s.API.REST("GET", "user", nil, nil, &user); err != nil {
		return "", err
	}
	if user.Login == "" {
		return "", errors.New("unable to determine authenticated user")
	}
	return user.Login, nil
}

func (s *Service) autoSubmitPendingReviews(pr resolver.Identity) error {
	login, err := s.currentLogin()
	if err != nil {
		return err
	}

	reviewSvc := review.NewService(s.API)
	summaries, _, err := reviewSvc.PendingSummaries(pr, review.PendingOptions{Reviewer: login, PerPage: 100})
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		return fmt.Errorf("no pending reviews owned by %s found on pull request #%d", login, pr.Number)
	}

	for _, pending := range summaries {
		path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews/%d/events", pr.Owner, pr.Repo, pr.Number, pending.DatabaseID)
		payload := map[string]interface{}{
			"event": "COMMENT",
			"body":  autoSubmitSummary,
		}
		if err := s.API.REST("POST", path, nil, payload, nil); err != nil {
			return err
		}
	}
	return nil
}
