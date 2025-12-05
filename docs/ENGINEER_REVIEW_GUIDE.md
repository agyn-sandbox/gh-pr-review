# Engineer Inline Comment Guide (gh-pr-review v1.3.1)

This guide helps engineers view and reply to inline pull request comments using the gh-pr-review extension. It reflects the current per-command backend policy and v1.3.1 behaviors.

## Requirements
- gh CLI installed and authenticated
- gh-pr-review v1.3.1
- Access to the target repository and PR

Check version:
```
gh extension list | grep gh-pr-review
```
Expected output shows `v1.3.1`.

### Install/Upgrade
- Upgrade: `gh extension upgrade Agyn-sandbox/gh-pr-review`
- If your platform lacks prebuilt binaries (e.g., linux-arm64), build from source:
```
# From source tag v1.3.1
git clone https://github.com/agyn-sandbox/gh-pr-review
cd gh-pr-review && git checkout v1.3.1
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o gh-pr-review .
cp gh-pr-review ~/.local/share/gh/extensions/gh-pr-review/gh-pr-review
sed -i 's/v1.3.0/v1.3.1/' ~/.local/share/gh/extensions/gh-pr-review/manifest.yml
```

## Backend Policy (one backend per command)
- GraphQL-only:
  - `review --start` (creates a pending review; returns PRR id and enriched fields)
  - `review --add-comment` (adds an inline review thread; returns thread fields)
  - `review pending-id` (fetches latest pending review id for a reviewer)
  - `review --submit` (status-only JSON; see below)
  - `threads list` / `threads resolve`
- REST-only:
  - `comments ids` / `comments reply`

Optional fields are omitted (not `null`). Commands never mix backends.

## Viewing Inline Comments
Use top-level PR view for context:
```
# Replace owner/repo and PR number
gh pr view -R owner/repo 123 --comments
```

List inline comment IDs for a specific reviewer’s latest review:
```
# Latest review by a reviewer; returns JSON array of comment objects
gh pr-review comments ids -R owner/repo --pr 123 --latest --reviewer reviewer_login
```
Output includes `id`, `body`, `user.login`, `html_url`, `path`, and optionally `line`.

## Replying Inline
Reply to each inline comment by ID:
```
# Standard reply (full payload)
gh pr-review comments reply -R owner/repo --pr 123 \
  --comment-id 2582545223 \
  --body "[Engineer] Acknowledged; will update."

# Concise reply (ID only)
gh pr-review comments reply -R owner/repo --pr 123 \
  --comment-id 2582545223 \
  --body "[Engineer] Ack (concise)." \
  --concise
```

## Reviewer Commands (for awareness)
While engineers typically don’t submit reviews, it’s helpful to understand reviewer flows:

Start a pending review (GraphQL-only):
```
# Creates a pending review; returns PRR id and enriched fields
gh pr-review review -R owner/repo --pr 123 --start
# Example output:
{"id":"PRR_...","state":"PENDING","database_id":3541...,"html_url":"https://github.com/...#pullrequestreview-..."}
```

Add inline comments (GraphQL-only) using the returned PRR id:
```
# Add a thread on a file and line
gh pr-review review -R owner/repo --pr 123 \
  --review-id PRR_... --add-comment \
  --path scenario.md --line 21 --side RIGHT \
  --body "Reviewer note"
# Example output:
{"id":"PRRT_...","path":"scenario.md","is_outdated":false,"line":21}
```

List unresolved threads (GraphQL-only):
```
# Returns [] when none
gh pr-review threads list -R owner/repo --pr 123 --unresolved
```

Resolve a thread (GraphQL-only):
```
# Reviewer action
gh pr-review threads resolve -R owner/repo --pr 123 --comment-id 2582545223
# Example output:
{"threadId":"PRRT_...","isResolved":true,"changed":true}
```

## Submitting a Review (Status-only)
`review --submit` uses a single GraphQL mutation and returns status-only JSON.

Event types (`--event`):
- `APPROVE` — no body required
- `REQUEST_CHANGES` — body required (explain requested changes)
- `COMMENT` — body required (general feedback)

Examples:
```
# Approve
gh pr-review review -R owner/repo --pr 123 \
  --review-id PRR_... --event APPROVE --submit

# Request changes
gh pr-review review -R owner/repo --pr 123 \
  --review-id PRR_... --event REQUEST_CHANGES \
  --body "Please address the noted issues." --submit

# Comment
gh pr-review review -R owner/repo --pr 123 \
  --review-id PRR_... --event COMMENT \
  --body "General feedback." --submit
```

- Success:
```
{"status":"Review submitted successfully"}
```
- Failure (non-zero exit):
```
{"status":"Review submission failed","errors":[{"message":"...","path":[...]}]}
```
Notes:
- `--review-id` must be a GraphQL review node id (format `PRR_...`).
- No follow-up fetch; enriched fields (id/state/submitted_at/database_id/html_url) are not returned.

## Tips
- If `pending-id` shows “pull request not found,” specify `--reviewer reviewer_login`.
- Optional fields are omitted; don’t expect `null` values.
- Linux-arm64 users may need to build from source; see Install/Upgrade.

## Example Workflow (Engineer)
1. List latest reviewer comments and capture IDs:
```
gh pr-review comments ids -R owner/repo --pr 123 --latest --reviewer reviewer_login
```
2. Reply to each comment:
```
gh pr-review comments reply -R owner/repo --pr 123 --comment-id <ID> --body "[Engineer] Ack."
```
3. Notify the reviewer in the PR thread or via your normal communication channel.

