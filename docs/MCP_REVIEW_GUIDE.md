# GitHub `gh` – Pull Request Review Guide

This reference provides standalone commands for agents that automate reviews
with the `gh pr-review` extension. Outputs are JSON-only, use field names that
mirror the GitHub REST/GraphQL APIs, and omit fields not present in the
upstream response.

## Selector Basics

- Use any of the supported selector formats: `-R owner/repo --pr 42`,
  `owner/repo#42`, or a pull request URL.
- Commands run from any directory; authentication and host selection follow the
  ambient `gh` CLI configuration (including `GH_HOST` for GitHub Enterprise
  Server).

## 1. Manage a Pending Review

Start or resume a pending review, add inline/file comments, then submit with the
desired state.

```sh
# Start or resume a pending review
gh pr-review review --start owner/repo#42

# Add an inline comment
gh pr-review review --add-comment \
  --review-id R_kwM123456789 \
  --path internal/service.go \
  --line 42 \
  --body "nit: use helper" \
  owner/repo#42

# Submit as COMMENT | APPROVE | REQUEST_CHANGES
gh pr-review review --submit \
  --review-id R_kwM123456789 \
  --event APPROVE \
  --body "Looks good" \
  owner/repo#42
```

Agents should capture the `review_id` returned from earlier steps (for example
via `review latest-id`) and reuse it in subsequent commands.

## 2. Read and Reply to Inline Comments

List comment identifiers (with text) for a review, then reply using the selected
ID.

```sh
# List comment IDs and bodies
gh pr-review comments ids --review_id 3531807471 --limit 50 owner/repo#42

# Reply to a specific comment
gh pr-review comments reply \
  --comment-id 2582545223 \
  --body "Thanks for catching this" \
  owner/repo#42
```

Agents should inspect the JSON array returned by `comments ids`, choose the
desired `id`, and provide it to `comments reply --comment-id`.

## 3. Resolve or Reopen Threads

Locate a thread by comment identifier, then resolve or unresolve it using the
thread node ID.

```sh
# Find the thread for a comment (returns { "id", "isResolved" })
gh pr-review threads find --comment_id 2582545223 owner/repo#42

# Resolve the thread
gh pr-review threads resolve --thread-id PRRT_kwDOQhKNiM5kbrOc owner/repo#42

# Reopen the thread if needed
gh pr-review threads unresolve --thread-id PRRT_kwDOQhKNiM5kbrOc owner/repo#42
```

Use the `id` returned from `threads find` as the argument for
`threads resolve` or `threads unresolve`. Alternatively, pass `--comment-id` to
resolve directly from the comment record.

## Error Handling Notes

- The extension surfaces GitHub API errors directly; agents should check for
  non-zero exit statuses and retry as necessary.
- Thread helpers automatically fall back to REST lookups when GraphQL fields
  are unavailable, ensuring compatibility with GitHub Enterprise Server.
