package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Agyn-sandbox/gh-pr-review/internal/ghcli"
	"github.com/Agyn-sandbox/gh-pr-review/internal/resolver"
)

// Service coordinates review GraphQL operations through the gh CLI.
type Service struct {
	API ghcli.API
}

// ReviewState contains metadata about a review after opening or submitting it.
type ReviewState struct {
	ID          string  `json:"id"`
	State       string  `json:"state"`
	SubmittedAt *string `json:"submitted_at"`
	DatabaseID  *int64  `json:"database_id,omitempty"`
	HTMLURL     string  `json:"html_url,omitempty"`
}

// ReviewThread represents an inline comment thread added to a pending review.
type ReviewThread struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	IsOutdated bool   `json:"is_outdated"`
}

// ThreadInput describes the inline comment details for AddThread.
type ThreadInput struct {
	ReviewID  string
	Path      string
	Line      int
	Side      string
	StartLine *int
	StartSide *string
	Body      string
}

// SubmitInput contains the payload for submitting a pending review.
type SubmitInput struct {
	ReviewID string
	Event    string
	Body     string
}

// NewService constructs a review Service.
func NewService(api ghcli.API) *Service {
	return &Service{API: api}
}

// Start opens a pending review for the specified pull request.
func (s *Service) Start(pr resolver.Identity, commitOID string) (*ReviewState, error) {
	nodeID, headSHA, err := s.pullRequestIdentifiers(pr)
	if err != nil {
		return nil, err
	}
	if commitOID == "" {
		commitOID = headSHA
	}

	query := `mutation AddPullRequestReview($input: AddPullRequestReviewInput!) {
  addPullRequestReview(input: $input) {
    pullRequestReview { id state submittedAt databaseId url }
  }
}`

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"pullRequestId": nodeID,
			"commitOID":     commitOID,
		},
	}

	var response struct {
		Data struct {
			AddPullRequestReview struct {
				PullRequestReview struct {
					ID          string  `json:"id"`
					State       string  `json:"state"`
					SubmittedAt *string `json:"submittedAt"`
					DatabaseID  *int64  `json:"databaseId"`
					URL         string  `json:"url"`
				} `json:"pullRequestReview"`
			} `json:"addPullRequestReview"`
		} `json:"data"`
	}

	if err := s.API.GraphQL(query, payload, &response); err != nil {
		return nil, err
	}

	review := response.Data.AddPullRequestReview.PullRequestReview
	state := ReviewState{
		ID:          review.ID,
		State:       review.State,
		SubmittedAt: review.SubmittedAt,
		DatabaseID:  review.DatabaseID,
		HTMLURL:     strings.TrimSpace(review.URL),
	}
	return &state, nil
}

// AddThread adds an inline review comment thread to an existing pending review.
func (s *Service) AddThread(pr resolver.Identity, input ThreadInput) (*ReviewThread, error) {
	if input.ReviewID == "" {
		return nil, errors.New("review id is required")
	}
	if input.Path == "" {
		return nil, errors.New("path is required")
	}
	if input.Line <= 0 {
		return nil, errors.New("line must be positive")
	}
	if input.Body == "" {
		return nil, errors.New("body is required")
	}

	query := `mutation AddPullRequestReviewThread($input: AddPullRequestReviewThreadInput!) {
  addPullRequestReviewThread(input: $input) {
    thread { id path isOutdated }
  }
}`

	graphqlInput := map[string]interface{}{
		"pullRequestReviewId": input.ReviewID,
		"path":                input.Path,
		"line":                input.Line,
		"side":                input.Side,
		"body":                input.Body,
	}
	if input.StartLine != nil {
		graphqlInput["startLine"] = *input.StartLine
	}
	if input.StartSide != nil {
		graphqlInput["startSide"] = *input.StartSide
	}

	payload := map[string]interface{}{
		"input": graphqlInput,
	}

	var response struct {
		Data struct {
			AddPullRequestReviewThread struct {
				Thread struct {
					ID         string `json:"id"`
					Path       string `json:"path"`
					IsOutdated bool   `json:"isOutdated"`
				} `json:"thread"`
			} `json:"addPullRequestReviewThread"`
		} `json:"data"`
	}

	if err := s.API.GraphQL(query, payload, &response); err != nil {
		return nil, err
	}

	thread := response.Data.AddPullRequestReviewThread.Thread
	return &ReviewThread{ID: thread.ID, Path: thread.Path, IsOutdated: thread.IsOutdated}, nil
}

// Submit finalizes a pending review with the given event and optional body.
func (s *Service) Submit(pr resolver.Identity, input SubmitInput) (*ReviewState, error) {
	if input.ReviewID == "" {
		return nil, errors.New("review id is required")
	}

	query := `mutation SubmitPullRequestReview($input: SubmitPullRequestReviewInput!) {
  submitPullRequestReview(input: $input) {
    pullRequestReview { id state submittedAt databaseId url }
  }
}`

	graphqlInput := map[string]interface{}{
		"pullRequestReviewId": input.ReviewID,
		"event":               input.Event,
	}
	if strings.TrimSpace(input.Body) != "" {
		graphqlInput["body"] = input.Body
	}

	payload := map[string]interface{}{
		"input": graphqlInput,
	}

	var response struct {
		Data struct {
			SubmitPullRequestReview struct {
				PullRequestReview json.RawMessage `json:"pullRequestReview"`
			} `json:"submitPullRequestReview"`
		} `json:"data"`
	}

	if err := s.API.GraphQL(query, payload, &response); err != nil {
		return nil, err
	}

	raw := response.Data.SubmitPullRequestReview.PullRequestReview
	trimmedRaw := bytes.TrimSpace(raw)
	if len(trimmedRaw) == 0 || bytes.Equal(trimmedRaw, []byte("null")) {
		return s.lookupLatestNonPendingByViewer(pr)
	}

	var review struct {
		ID          string  `json:"id"`
		State       string  `json:"state"`
		SubmittedAt *string `json:"submittedAt"`
		DatabaseID  *int64  `json:"databaseId"`
		URL         string  `json:"url"`
	}
	if err := json.Unmarshal(trimmedRaw, &review); err != nil {
		return nil, fmt.Errorf("decode submit review: %w", err)
	}

	reviewID := strings.TrimSpace(review.ID)
	if reviewID == "" {
		return nil, errors.New("submit review response missing review id")
	}

	var submittedAt *string
	if review.SubmittedAt != nil {
		trimmed := strings.TrimSpace(*review.SubmittedAt)
		if trimmed != "" {
			submittedAt = &trimmed
		}
	}

	stateValue := strings.TrimSpace(review.State)
	htmlURL := strings.TrimSpace(review.URL)

	state := ReviewState{
		ID:          reviewID,
		State:       stateValue,
		SubmittedAt: submittedAt,
		DatabaseID:  review.DatabaseID,
		HTMLURL:     htmlURL,
	}
	return &state, nil
}

func (s *Service) lookupLatestNonPendingByViewer(pr resolver.Identity) (*ReviewState, error) {
	login, err := s.currentViewer()
	if err != nil {
		return nil, err
	}

	const query = `query LatestNonPendingReview($owner: String!, $name: String!, $number: Int!, $author: String!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviews(last: 1, author: $author, states: [APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED]) {
        nodes {
          id
          state
          submittedAt
          databaseId
          url
        }
      }
    }
  }
}`

	variables := map[string]interface{}{
		"owner":  pr.Owner,
		"name":   pr.Repo,
		"number": pr.Number,
		"author": login,
	}

	var response struct {
		Data struct {
			Repository *struct {
				PullRequest *struct {
					Reviews *struct {
						Nodes []struct {
							ID          string  `json:"id"`
							State       string  `json:"state"`
							SubmittedAt *string `json:"submittedAt"`
							DatabaseID  *int64  `json:"databaseId"`
							URL         string  `json:"url"`
						} `json:"nodes"`
					} `json:"reviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := s.API.GraphQL(query, variables, &response); err != nil {
		return nil, err
	}

	repo := response.Data.Repository
	if repo == nil || repo.PullRequest == nil || repo.PullRequest.Reviews == nil {
		return nil, fmt.Errorf("pull request %s/%s#%d not found", pr.Owner, pr.Repo, pr.Number)
	}

	if len(repo.PullRequest.Reviews.Nodes) == 0 {
		return nil, fmt.Errorf("no submitted reviews for %s", login)
	}

	review := repo.PullRequest.Reviews.Nodes[0]
	reviewID := strings.TrimSpace(review.ID)
	if reviewID == "" {
		return nil, errors.New("latest review missing id")
	}

	var submittedAt *string
	if review.SubmittedAt != nil {
		trimmed := strings.TrimSpace(*review.SubmittedAt)
		if trimmed != "" {
			submittedAt = &trimmed
		}
	}

	state := ReviewState{
		ID:          reviewID,
		State:       strings.TrimSpace(review.State),
		SubmittedAt: submittedAt,
		DatabaseID:  review.DatabaseID,
		HTMLURL:     strings.TrimSpace(review.URL),
	}
	return &state, nil
}

func (s *Service) currentViewer() (string, error) {
	const query = `query ViewerLogin { viewer { login } }`

	var response struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
		} `json:"data"`
	}

	if err := s.API.GraphQL(query, nil, &response); err != nil {
		return "", err
	}

	login := strings.TrimSpace(response.Data.Viewer.Login)
	if login == "" {
		return "", errors.New("viewer login unavailable")
	}

	return login, nil
}

func (s *Service) pullRequestIdentifiers(pr resolver.Identity) (string, string, error) {
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", pr.Owner, pr.Repo, pr.Number)
	var data struct {
		NodeID string `json:"node_id"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := s.API.REST("GET", path, nil, nil, &data); err != nil {
		return "", "", err
	}
	if data.NodeID == "" || data.Head.SHA == "" {
		return "", "", errors.New("pull request metadata incomplete")
	}
	return data.NodeID, data.Head.SHA, nil
}
