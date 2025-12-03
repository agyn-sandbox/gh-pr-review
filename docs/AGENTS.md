# Agent Usage with `gh pr-review`

This guide provides ready-to-run prompts for scripted or agent-driven use of
`gh pr-review`. All commands emit JSON with REST/GraphQL-aligned field names and
include only values present in upstream responses (no null placeholders).

## 1. Review a pull request end-to-end

```sh
# Start or resume a pending review for PR 42
gh pr-review review --start owner/repo#42

# Add an inline comment to the pending review
gh pr-review review --add-comment \
  --review-id R_kwM123456 \
  --path internal/service.go \
  --line 42 \
  --body "nit: use helper" \
  owner/repo#42

# Submit the review (COMMENT | APPROVE | REQUEST_CHANGES)
gh pr-review review --submit \
  --review-id R_kwM123456 \
  --event APPROVE \
  --body "Looks good" \
  owner/repo#42
```

## 2. Read and reply to inline comments

```sh
# List comment identifiers (IDs + text) for a specific review
gh pr-review comments ids --review_id 3531807471 --limit 20 owner/repo#42

# Reply to a comment by database identifier
gh pr-review comments reply \
  --comment-id 2582545223 \
  --body "Thanks for catching this" \
  owner/repo#42
```

Chained sequence:

```sh
# 1) Capture the latest review ID for octocat
review_id=$(gh pr-review review latest-id --reviewer octocat owner/repo#42 | jq '.id')

# 2) List comments and pick one
comment_id=$(gh pr-review comments ids --review_id "$review_id" owner/repo#42 | jq '.[0].id')

# 3) Reply to that comment
gh pr-review comments reply --comment-id "$comment_id" --body "Updated." owner/repo#42
```

## 3. Resolve or reopen discussion threads

```sh
# Locate the thread for a specific comment; emits { "id", "isResolved" }
thread_json=$(gh pr-review threads find --comment_id 2582545223 owner/repo#42)
thread_id=$(echo "$thread_json" | jq -r '.id')

# Resolve the thread
gh pr-review threads resolve --thread-id "$thread_id" owner/repo#42

# Reopen the thread if needed
gh pr-review threads unresolve --thread-id "$thread_id" owner/repo#42
```

The thread commands also accept `--comment-id` to resolve directly from a REST
comment identifier; each response reflects only the fields returned by GitHub.
