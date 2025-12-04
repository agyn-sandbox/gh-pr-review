package review

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

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

func TestServiceStart(t *testing.T) {
	api := &fakeAPI{}
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		if path == "repos/octo/demo/pulls/7" {
			payload := map[string]interface{}{"node_id": "PR_node", "head": map[string]interface{}{"sha": "abc123"}}
			return assign(result, payload)
		}
		return errors.New("unexpected path")
	}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"addPullRequestReview": map[string]interface{}{
					"pullRequestReview": map[string]interface{}{
						"id":          "RV1",
						"state":       "PENDING",
						"submittedAt": nil,
						"databaseId":  321,
						"url":         "https://example.com/review/RV1",
					},
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	state, err := svc.Start(pr, "")
	require.NoError(t, err)
	assert.Equal(t, "RV1", state.ID)
	assert.Equal(t, "PENDING", state.State)
	require.NotNil(t, state.DatabaseID)
	assert.Equal(t, int64(321), *state.DatabaseID)
	assert.Equal(t, "https://example.com/review/RV1", state.HTMLURL)
}

func TestServiceAddThread(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"addPullRequestReviewThread": map[string]interface{}{
					"thread": map[string]interface{}{"id": "THREAD1", "path": "file.go", "isOutdated": false},
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	thread, err := svc.AddThread(pr, ThreadInput{ReviewID: "RV1", Path: "file.go", Line: 10, Side: "RIGHT", Body: "note"})
	require.NoError(t, err)
	assert.Equal(t, "THREAD1", thread.ID)
}

func TestServiceSubmit(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"submitPullRequestReview": map[string]interface{}{
					"pullRequestReview": map[string]interface{}{
						"id":          " RV1 ",
						"state":       "COMMENTED ",
						"submittedAt": " 2024-05-01T12:00:00Z ",
						"databaseId":  654,
						"url":         "https://example.com/review/RV1 ",
					},
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	state, err := svc.Submit(pr, SubmitInput{ReviewID: "RV1", Event: "COMMENT"})
	require.NoError(t, err)
	assert.Equal(t, "RV1", state.ID)
	assert.Equal(t, "COMMENTED", state.State)
	require.NotNil(t, state.SubmittedAt)
	assert.Equal(t, "2024-05-01T12:00:00Z", *state.SubmittedAt)
	require.NotNil(t, state.DatabaseID)
	assert.Equal(t, int64(654), *state.DatabaseID)
	assert.Equal(t, "https://example.com/review/RV1", state.HTMLURL)
}

func TestServiceSubmitFallsBackToViewerLookup(t *testing.T) {
	api := &fakeAPI{}
	call := 0
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		call++
		switch {
		case strings.Contains(query, "SubmitPullRequestReview"):
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"submitPullRequestReview": map[string]interface{}{
						"pullRequestReview": nil,
					},
				},
			}
			return assign(result, payload)
		case strings.Contains(query, "ViewerLogin"):
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"viewer": map[string]interface{}{
						"login": "casey",
					},
				},
			}
			return assign(result, payload)
		case strings.Contains(query, "LatestNonPendingReview"):
			require.Equal(t, "octo", variables["owner"])
			require.Equal(t, "demo", variables["name"])
			require.EqualValues(t, 7, variables["number"])
			require.Equal(t, "casey", variables["author"])
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequest": map[string]interface{}{
							"reviews": map[string]interface{}{
								"nodes": []map[string]interface{}{
									{
										"id":          " RV99 ",
										"state":       " APPROVED ",
										"submittedAt": "2024-07-01T09:00:00Z ",
										"databaseId":  2024,
										"url":         " https://example.com/review/RV99 ",
									},
								},
							},
						},
					},
				},
			}
			return assign(result, payload)
		default:
			return errors.New("unexpected query: " + query)
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	state, err := svc.Submit(pr, SubmitInput{ReviewID: "RV1", Event: "COMMENT"})
	require.NoError(t, err)
	assert.Equal(t, "RV99", state.ID)
	assert.Equal(t, "APPROVED", state.State)
	require.NotNil(t, state.SubmittedAt)
	assert.Equal(t, "2024-07-01T09:00:00Z", *state.SubmittedAt)
	require.NotNil(t, state.DatabaseID)
	assert.Equal(t, int64(2024), *state.DatabaseID)
	assert.Equal(t, "https://example.com/review/RV99", state.HTMLURL)
	assert.Equal(t, 3, call)
}

func TestServiceSubmitFallbackMissingReview(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		switch {
		case strings.Contains(query, "SubmitPullRequestReview"):
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"submitPullRequestReview": map[string]interface{}{
						"pullRequestReview": nil,
					},
				},
			}
			return assign(result, payload)
		case strings.Contains(query, "ViewerLogin"):
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"viewer": map[string]interface{}{
						"login": "octocat",
					},
				},
			}
			return assign(result, payload)
		case strings.Contains(query, "LatestNonPendingReview"):
			payload := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequest": map[string]interface{}{
							"reviews": map[string]interface{}{
								"nodes": []map[string]interface{}{},
							},
						},
					},
				},
			}
			return assign(result, payload)
		default:
			return errors.New("unexpected query: " + query)
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Submit(pr, SubmitInput{ReviewID: "RV1", Event: "APPROVE"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no submitted reviews for octocat")
}

func TestServiceSubmitErrorOnMissingReviewID(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"submitPullRequestReview": map[string]interface{}{
					"pullRequestReview": map[string]interface{}{
						"id":          " ",
						"state":       "PENDING",
						"submittedAt": nil,
						"databaseId":  123,
						"url":         "https://example.com/review",
					},
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Submit(pr, SubmitInput{ReviewID: "RV1", Event: "APPROVE"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing review id")
}

func assign(result interface{}, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}
