# MCP Review Guide (gh-pr-review v1.3.1)

This guide describes how to run end-to-end pull request reviews using the gh-pr-review extension with strict single-backend behavior per command. It is suitable for scripted flows (MCP, automation) and manual use.

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


## Command behavior quick reference
- review --start: create pending review → returns { id, state, database_id?, html_url? }
- review --add-comment: add inline thread → returns { id, path, is_outdated, line? }
- review pending-id: latest pending review id for reviewer
- review --submit: status-only JSON
  - Success: { "status": "Review submitted successfully" }
  - Failure: { "status": "Review submission failed", "errors": [ { "message": "...", "path": [ ... ] } ] }
- threads list --unresolved: [] when none
- threads resolve: { threadId, isResolved: true, changed: true }
- comments ids/reply: list IDs; reply by ID (supports --concise)

Notes
- Use --reviewer with pending-id when needed.
- Optional fields omitted (not null).
- Submit events (--event): APPROVE (no body), REQUEST_CHANGES (body required), COMMENT (body required).
## Core Review Flow (Scriptable)
Use these steps to script an end-to-end PR review.

1) Start a pending review (GraphQL-only)
```
# Returns PRR id and enriched fields
gh pr-review review -R owner/repo --pr 123 --start
# Example output:
{"id":"PRR_...","state":"PENDING","database_id":3541...,"html_url":"https://github.com/...#pullrequestreview-..."}
```
Capture the `id` (PRR_...) for subsequent commands.

2) Add inline comments (GraphQL-only)
```
# Add a thread to a file at a specific line and side
gh pr-review review -R owner/repo --pr 123 \
  --review-id PRR_... --add-comment \
  --path scenario.md --line 21 --side RIGHT \
  --body "Reviewer note"
# Example output:
{"id":"PRRT_...","path":"scenario.md","is_outdated":false,"line":21}
```
Repeat per comment.

3) List unresolved threads (GraphQL-only)
```
# Returns [] when none
gh pr-review threads list -R owner/repo --pr 123 --unresolved
```

4) Resolve a thread (GraphQL-only)
```
# Mark an inline thread as resolved
gh pr-review threads resolve -R owner/repo --pr 123 --comment-id <INLINE_COMMENT_ID>
# Example output:
{"threadId":"PRRT_...","isResolved":true,"changed":true}
```

5) Submit review (Status-only, GraphQL-only)
`review --submit` uses one GraphQL mutation and returns status-only JSON.

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

## Comment Utilities (for Engineers)
List the latest inline comment IDs for a reviewer and reply:
```
# IDs for latest review by a reviewer
gh pr-review comments ids -R owner/repo --pr 123 --latest --reviewer reviewer_login

# Reply (full payload)
gh pr-review comments reply -R owner/repo --pr 123 --comment-id <ID> --body "[Engineer] Ack."

# Reply (concise)
gh pr-review comments reply -R owner/repo --pr 123 --comment-id <ID> --body "[Engineer] Ack (concise)." --concise
```

## Tips
- If `pending-id` returns “pull request not found,” pass `--reviewer reviewer_login`.
- Optional fields are omitted; never expect `null` placeholders.
- linux-arm64 users should build from source; see Install/Upgrade.

## Example: Full Script Outline
- Start → Add comments → List unresolved → Resolve → Submit
- Capture outputs at each step and persist the PRR id for subsequent commands.
- Use status-only submit to determine success or failure without additional fetches.
