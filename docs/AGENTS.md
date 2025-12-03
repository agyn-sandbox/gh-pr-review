# Agent Usage with `gh pr-review`

This guide provides ready-to-run prompts for agents or scripted workflows that
use the `gh pr-review` extension. All commands output JSON with field names
matching the GitHub REST/GraphQL APIs and include only data present in the
source responses (no null placeholders).

## Setup and Selector Patterns

- Include the repository via `-R owner/repo`, a pull request URL, or the
  shorthand `owner/repo#123`.
- The extension works from any directory; no local git checkout is required.
- Authentication and host selection reuse the ambient `gh` CLI configuration,
  including `GH_HOST` for GitHub Enterprise Server.

## Review Workflow (E2E)

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

# Submit the review with a chosen event
gh pr-review review --submit \
  --review-id R_kwM123456 \
  --event COMMENT \
  --body "Looks good" \
  owner/repo#42
```

## Comments

```sh
# List comment identifiers (with bodies) for a review
gh pr-review comments ids --review_id 3531807471 --limit 50 owner/repo#42

# Reply to a comment by identifier (auto-submits pending reviews if needed)
gh pr-review comments reply --comment-id 987654 --body "Thanks" owner/repo#42
```

## Threads

```sh
# Find a thread by comment ID or thread ID; emits { "id", "isResolved" }
 gh pr-review threads find --comment_id 2582545223 owner/repo#42

# List unresolved threads (JSON array of threads)
 gh pr-review threads list --unresolved owner/repo#42

# Resolve or unresolve a thread by node ID or comment ID
 gh pr-review threads resolve --thread-id R_ywDoABC123 owner/repo#42
 gh pr-review threads resolve --comment-id 987654 owner/repo#42
 gh pr-review threads unresolve --thread-id R_ywDoABC123 owner/repo#42
```

## Error Handling and GHES Notes

- Commands surface upstream API errors directly and abort on unexpected states.
- Thread operations fall back to REST lookups when GraphQL fields are absent,
  enabling compatibility with GitHub Enterprise Server.
- Selector parsing normalizes URLs, PR numbers, and shorthand patterns; invalid
  selectors return actionable error messages.
