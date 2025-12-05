package comments

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Agyn-sandbox/gh-pr-review/internal/ghcli"
	"github.com/Agyn-sandbox/gh-pr-review/internal/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAPI struct {
	restFunc    func(method, path string, params map[string]string, body interface{}, result interface{}) error
	graphqlFunc func(query string, variables map[string]interface{}, result interface{}) error
}

func (f *fakeAPI) REST(method, path string, params map[string]string, body interface{}, result interface{}) error {
	if f.restFunc == nil {
		return errors.New("unexpected REST call")
	}
	return f.restFunc(method, path, params, body, result)
}

func (f *fakeAPI) GraphQL(query string, variables map[string]interface{}, result interface{}) error {
	if f.graphqlFunc == nil {
		return errors.New("unexpected GraphQL call")
	}
	return f.graphqlFunc(query, variables, result)
}

func assign(result interface{}, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

type pendingReviewNode struct {
	ID                string `json:"id"`
	DatabaseID        int64  `json:"databaseId"`
	State             string `json:"state"`
	AuthorAssociation string `json:"authorAssociation"`
	URL               string `json:"url"`
	UpdatedAt         string `json:"updatedAt"`
	CreatedAt         string `json:"createdAt"`
	Author            struct {
		Login      string `json:"login"`
		DatabaseID int64  `json:"databaseId"`
	} `json:"author"`
}

func TestServiceReply_RejectsInvalidCommentID(t *testing.T) {
	api := &fakeAPI{}
	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}

	_, err := svc.Reply(pr, ReplyOptions{CommentID: 0, Body: "hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comment id")
}

func TestServiceReply_RejectsBlankBody(t *testing.T) {
	api := &fakeAPI{}
	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}

	_, err := svc.Reply(pr, ReplyOptions{CommentID: 5, Body: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reply body is required")
}

func TestServiceReply_AutoSubmitPending(t *testing.T) {
	api := &fakeAPI{}
	var submitted []int64
	attempt := 0
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		if !strings.Contains(query, "PendingReviews") {
			return errors.New("unexpected query: " + query)
		}
		require.Equal(t, "octo", variables["owner"])
		require.Equal(t, "demo", variables["name"])
		require.EqualValues(t, 7, variables["number"])
		require.EqualValues(t, 100, variables["pageSize"])
		_, hasCursor := variables["cursor"]
		require.False(t, hasCursor)
		_, hasAuthor := variables["author"]
		require.False(t, hasAuthor)

		payload := struct {
			Data struct {
				Viewer struct {
					Login      string `json:"login"`
					DatabaseID int64  `json:"databaseId"`
				} `json:"viewer"`
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes    []pendingReviewNode `json:"nodes"`
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"reviews"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}{}
		payload.Data.Viewer.Login = "casey"
		payload.Data.Viewer.DatabaseID = 77
		payload.Data.Repository.PullRequest.Reviews.Nodes = []pendingReviewNode{
			{
				ID:                "RV_pending_99",
				DatabaseID:        99,
				State:             "PENDING",
				AuthorAssociation: "MEMBER",
				URL:               "https://example.com/review/99",
				UpdatedAt:         "2024-06-02T12:00:00Z",
				CreatedAt:         "2024-06-02T11:45:00Z",
				Author: struct {
					Login      string `json:"login"`
					DatabaseID int64  `json:"databaseId"`
				}{Login: "octocat", DatabaseID: 202},
			},
		}
		payload.Data.Repository.PullRequest.Reviews.PageInfo.HasNextPage = false
		payload.Data.Repository.PullRequest.Reviews.PageInfo.EndCursor = ""
		return assign(result, payload)
	}
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		switch {
		case path == "user":
			return assign(result, map[string]interface{}{"login": "octocat"})
		case path == "repos/octo/demo/pulls/7/reviews":
			payload := []map[string]interface{}{
				{"id": 99, "state": "PENDING", "user": map[string]interface{}{"login": "octocat"}},
			}
			return assign(result, payload)
		case path == "repos/octo/demo/pulls/7/reviews/99/events" && method == "POST":
			submitted = append(submitted, 99)
			return nil
		case path == "repos/octo/demo/pulls/7/comments/5/replies" && method == "POST":
			attempt++
			if attempt == 1 {
				return &ghcli.APIError{
					StatusCode: 422,
					Message:    "gh: Validation Failed (HTTP 422)",
					Body:       `{"message":"Validation Failed","errors":[{"message":"user_id can only have one pending review per pull request"}]}`,
				}
			}
			return assign(result, map[string]interface{}{"id": 123, "body": "ok"})
		default:
			return errors.New("unexpected request: " + path)
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	reply, err := svc.Reply(pr, ReplyOptions{CommentID: 5, Body: "ack"})
	require.NoError(t, err)
	assert.Contains(t, string(reply), "\"id\":123")
	assert.Equal(t, []int64{99}, submitted)
}

func TestServiceReply_PendingMissing(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		if !strings.Contains(query, "PendingReviews") {
			return errors.New("unexpected query")
		}
		require.Equal(t, "octo", variables["owner"])
		require.Equal(t, "demo", variables["name"])
		require.EqualValues(t, 7, variables["number"])
		require.EqualValues(t, 100, variables["pageSize"])
		_, hasCursor := variables["cursor"]
		require.False(t, hasCursor)
		_, hasAuthor := variables["author"]
		require.False(t, hasAuthor)

		payload := struct {
			Data struct {
				Viewer struct {
					Login      string `json:"login"`
					DatabaseID int64  `json:"databaseId"`
				} `json:"viewer"`
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes    []pendingReviewNode `json:"nodes"`
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"reviews"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}{}
		payload.Data.Viewer.Login = "casey"
		payload.Data.Viewer.DatabaseID = 77
		payload.Data.Repository.PullRequest.Reviews.Nodes = nil
		payload.Data.Repository.PullRequest.Reviews.PageInfo.HasNextPage = false
		payload.Data.Repository.PullRequest.Reviews.PageInfo.EndCursor = ""
		return assign(result, payload)
	}
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		switch path {
		case "repos/octo/demo/pulls/7/comments/5/replies":
			return &ghcli.APIError{
				StatusCode: 422,
				Message:    "gh: Validation Failed (HTTP 422)",
				Body:       `{"message":"Validation Failed","errors":[{"message":"user_id can only have one pending review per pull request"}]}`,
			}
		case "user":
			return assign(result, map[string]interface{}{"login": "octocat"})
		case "repos/octo/demo/pulls/7/reviews":
			return assign(result, []map[string]interface{}{})
		default:
			return errors.New("unexpected path")
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Reply(pr, ReplyOptions{CommentID: 5, Body: "ack"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pending reviews for octocat")
}
