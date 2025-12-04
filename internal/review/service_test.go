package review

import (
	"encoding/json"
	"errors"
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
	call := 0
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		call++
		switch call {
		case 1:
			assert.Contains(t, query, "pullRequest(number:$number)")
			assert.Equal(t, "octo", variables["owner"])
			assert.Equal(t, "demo", variables["name"])
			assert.Equal(t, 7, variables["number"])
			payload := map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequest": map[string]interface{}{
						"id":         "PRR_node",
						"headRefOid": "abc123",
					},
				},
			}
			return assign(result, payload)
		case 2:
			assert.Contains(t, query, "addPullRequestReview")
			input, ok := variables["input"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "PRR_node", input["pullRequestId"])
			assert.Equal(t, "abc123", input["commitOID"])
			payload := map[string]interface{}{
				"addPullRequestReview": map[string]interface{}{
					"pullRequestReview": map[string]interface{}{
						"id":          "PRR_review",
						"state":       "PENDING",
						"submittedAt": nil,
						"databaseId":  321,
						"url":         "https://example.com/review/PRR_review",
					},
				},
			}
			return assign(result, payload)
		default:
			return errors.New("unexpected GraphQL call")
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	state, err := svc.Start(pr, "")
	require.NoError(t, err)
	assert.Equal(t, "PRR_review", state.ID)
	assert.Equal(t, "PENDING", state.State)
	require.NotNil(t, state.DatabaseID)
	assert.Equal(t, int64(321), *state.DatabaseID)
	require.Nil(t, state.SubmittedAt)
	require.NotNil(t, state.HTMLURL)
	assert.Equal(t, "https://example.com/review/PRR_review", *state.HTMLURL)
	assert.Equal(t, 2, call)
}

func TestServiceStartErrorOnEmptyReview(t *testing.T) {
	api := &fakeAPI{}
	step := 0
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		step++
		switch step {
		case 1:
			payload := map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequest": map[string]interface{}{
						"id":         "PRR_node",
						"headRefOid": "abc123",
					},
				},
			}
			return assign(result, payload)
		case 2:
			payload := map[string]interface{}{
				"addPullRequestReview": map[string]interface{}{
					"pullRequestReview": map[string]interface{}{
						"id": " ",
					},
				},
			}
			return assign(result, payload)
		default:
			return errors.New("unexpected GraphQL call")
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Start(pr, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "addPullRequestReview returned no review data")
}

func TestServiceAddThread(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		assert.Contains(t, query, "addPullRequestReviewThread")
		input, ok := variables["input"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "PRR_review", input["pullRequestReviewId"])
		assert.Equal(t, "file.go", input["path"])
		assert.Equal(t, 10, input["line"])
		assert.Equal(t, "RIGHT", input["side"])
		assert.Equal(t, "note", input["body"])

		payload := map[string]interface{}{
			"addPullRequestReviewThread": map[string]interface{}{
				"thread": map[string]interface{}{
					"id":         "THR1",
					"path":       "file.go",
					"isOutdated": false,
					"line":       10,
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	thread, err := svc.AddThread(pr, ThreadInput{ReviewID: " PRR_review ", Path: " file.go ", Line: 10, Side: "RIGHT", Body: " note "})
	require.NoError(t, err)
	assert.Equal(t, "THR1", thread.ID)
	assert.Equal(t, "file.go", thread.Path)
	assert.False(t, thread.IsOutdated)
	require.NotNil(t, thread.Line)
	assert.Equal(t, 10, *thread.Line)
}

func TestServiceAddThreadErrorsOnIncompleteResponse(t *testing.T) {
	api := &fakeAPI{}
	api.graphqlFunc = func(query string, variables map[string]interface{}, result interface{}) error {
		payload := map[string]interface{}{
			"addPullRequestReviewThread": map[string]interface{}{
				"thread": map[string]interface{}{
					"id":   "",
					"path": "file.go",
				},
			},
		}
		return assign(result, payload)
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.AddThread(pr, ThreadInput{ReviewID: "PRR_review", Path: "file.go", Line: 10, Side: "RIGHT", Body: "note"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned incomplete thread data")
}

func TestServiceAddThreadRequiresGraphQLReviewID(t *testing.T) {
	api := &fakeAPI{}
	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}

	_, err := svc.AddThread(pr, ThreadInput{ReviewID: "511", Path: "file.go", Line: 10, Side: "RIGHT", Body: "note"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GraphQL node id")
}

func TestServiceAddThreadRequiresPath(t *testing.T) {
	api := &fakeAPI{}
	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}

	_, err := svc.AddThread(pr, ThreadInput{ReviewID: "PRR_review", Path: "", Line: 10, Side: "RIGHT", Body: "note"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

func TestServiceSubmit(t *testing.T) {
	api := &fakeAPI{}
	call := 0
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		call++
		switch call {
		case 1:
			require.Equal(t, "POST", method)
			require.Equal(t, "repos/octo/demo/pulls/7/reviews/511/events", path)
			require.Nil(t, params)
			require.IsType(t, map[string]string{}, body)
			payload := body.(map[string]string)
			require.Equal(t, "COMMENT", payload["event"])
			require.Equal(t, "Looks good", payload["body"])
			return nil
		case 2:
			require.Equal(t, "GET", method)
			require.Equal(t, "repos/octo/demo/pulls/7/reviews/511", path)
			require.Nil(t, params)
			response := map[string]interface{}{
				"id":           511,
				"node_id":      " RV1 ",
				"state":        " COMMENTED ",
				"submitted_at": " 2024-05-01T12:00:00Z ",
				"html_url":     " https://example.com/review/RV1 ",
			}
			return assign(result, response)
		default:
			return errors.New("unexpected REST call")
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	state, err := svc.Submit(pr, SubmitInput{ReviewID: " 511 ", Event: "COMMENT", Body: " Looks good "})
	require.NoError(t, err)
	assert.Equal(t, "RV1", state.ID)
	assert.Equal(t, "COMMENTED", state.State)
	require.NotNil(t, state.SubmittedAt)
	assert.Equal(t, "2024-05-01T12:00:00Z", *state.SubmittedAt)
	require.NotNil(t, state.DatabaseID)
	assert.Equal(t, int64(511), *state.DatabaseID)
	require.NotNil(t, state.HTMLURL)
	assert.Equal(t, "https://example.com/review/RV1", *state.HTMLURL)
	assert.Equal(t, 2, call)
}

func TestServiceSubmitErrorOnMissingReviewID(t *testing.T) {
	api := &fakeAPI{}
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		t.Fatalf("unexpected REST call: %s %s", method, path)
		return nil
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Submit(pr, SubmitInput{ReviewID: " ", Event: "APPROVE"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "review id is required")
}

func TestServiceSubmitErrorOnNonNumericReviewID(t *testing.T) {
	api := &fakeAPI{}
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		t.Fatalf("unexpected REST call: %s %s", method, path)
		return nil
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Submit(pr, SubmitInput{ReviewID: "RV1", Event: "COMMENT"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be numeric")
}

func TestServiceSubmitErrorOnMissingNodeID(t *testing.T) {
	api := &fakeAPI{}
	call := 0
	api.restFunc = func(method, path string, params map[string]string, body interface{}, result interface{}) error {
		call++
		switch call {
		case 1:
			return nil
		case 2:
			payload := map[string]interface{}{
				"id":           511,
				"node_id":      " ",
				"state":        "COMMENTED",
				"submitted_at": nil,
				"html_url":     "https://example.com/review/RV1",
			}
			return assign(result, payload)
		default:
			return errors.New("unexpected REST call")
		}
	}

	svc := NewService(api)
	pr := resolver.Identity{Owner: "octo", Repo: "demo", Number: 7, Host: "github.com"}
	_, err := svc.Submit(pr, SubmitInput{ReviewID: "511", Event: "COMMENT"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing node identifier")
}

func assign(result interface{}, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}
