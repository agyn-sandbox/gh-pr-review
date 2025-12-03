# GitHub gh – Pull Request Review Guide
> Commands used: `gh pr review --start`, `gh pr review --add-comment`, `gh pr review --submit`, `gh pr-review comments ids`, `gh pr-review comments reply`, `gh pr-review threads find`, `gh pr-review threads resolve`, `gh pr-review threads unresolve`

This workflow creates a pending review, adds one or more review comments (inline or file-level), and then submits the review with a final decision. All outputs are JSON-only by default; no `--json` flag is required.

# Documentation: Using `gh` to Review Pull Requests
> Use these commands to review pull requests in the repository via `gh`.

## View PR details
- Show PR title, description, and metadata
  - `gh pr view <number>`
- Show PR with all top-level comments
  - `gh pr view <number> --comments`
- View structured PR data as JSON
  - Fields available for `--json` include: files, reviews, headRefOid, baseRefName, headRefName, author, body, comments
  - `gh pr view <number> --json files,reviews,headRefOid --jq '.files[].path'`

## Inspect changes
- Show full diff of code changes
  - `gh pr diff <number>`
- Show only names of changed files
  - `gh pr diff <number> --name-only`

## 1. Create a pending review
Command:
- `gh pr review --start -R owner/repo --pr N`

Required:
- `-R owner/repo` – repository selector
- `--pr N` – pull request number

Notes:
- Call this once to start a review session. If a pending review already exists, skip this and go to step 2.

## 2. Add comments to the pending review
Command (inline or file-level):
- Inline (single line):
  - `gh pr review --add-comment -R owner/repo --pr N --path path/to/file.go --line 45 --body "[minor] Consider renaming this variable for clarity."`
- Inline (multi-line range):
  - `gh pr review --add-comment -R owner/repo --pr N --path path/to/file.go --start-line 40 --line 48 --body "[nit] Extract this repeated logic into a helper function."`
- File-level:
  - `gh pr review --add-comment -R owner/repo --pr N --path path/to/file.go --body "[major] Please document this module more clearly."`

Notes:
- Every comment body must start with a level indicator: `[major]`, `[minor]`, or `[nit]`.
- Repeat the add-comment command for each comment you want to include in the same pending review.
- Use “RIGHT” side implicitly for changed lines in the PR diff (side selection handled internally).

## 2a. Commenting Guidelines
- Use one comment per distinct issue.
- Keep ranges as short as possible (ideally ≤5–10 lines) to pinpoint the problem.
- Use Markdown formatting in comment bodies.
- Ensure each comment begins with `[major]`, `[minor]`, or `[nit]` to indicate severity/urgency.

## 3. Submit the pending review
Command:
- `gh pr review --submit -R owner/repo --pr N --event APPROVE --body "Looks good to me."`
- `gh pr review --submit -R owner/repo --pr N --event REQUEST_CHANGES --body "Please address the inline comments and add missing tests."`
- `gh pr review --submit -R owner/repo --pr N --event COMMENT --body "Added general feedback; see comments for details."`

Required:
- `--event` – one of `APPROVE`, `REQUEST_CHANGES`, `COMMENT`

Additional Rule:
- If any `[major]` level problem is present in the review comments, you must submit with `REQUEST_CHANGES`.

## 4. Review workflow summary
1) `gh pr review --start -R owner/repo --pr N` – start a new pending review.
2) `gh pr review --add-comment ...` – add inline or file-level comments (each body begins with [major]/[minor]/[nit]).
3) `gh pr review --submit ...` – finalize with `APPROVE`, `REQUEST_CHANGES`, or `COMMENT`.

## Inline comments: list IDs and reply
- List review comments (IDs + text) for latest review by a reviewer:
  - `gh pr-review comments ids -R owner/repo --pr N --latest --reviewer reviewer_login`
- Or for a specific review:
  - `gh pr-review comments ids owner/repo#N --review_id 3531807471`
- Reply to a specific comment (use returned ID):
  - `gh pr-review comments reply -R owner/repo --pr N --comment-id 2582545223 --body "[minor] Acknowledged; will add retry logic."`

Behavior:
- Output includes `id`, `body`, `user` (login/id), timestamps, `html_url`, and `path/line` when available.
- Pagination: `--per_page`, `--page`, `--limit`.

## Threads: find from comment and resolve/unresolve
- Find a thread from a comment ID (returns minimal schema):
  - `gh pr-review threads find -R owner/repo --pr N --comment_id 2582545223`
  - Output: `{ "id": "PRRT_...", "isResolved": false }`
- Resolve a thread (use returned thread id):
  - `gh pr-review threads resolve -R owner/repo --pr N --thread-id PRRT_...`
- Unresolve a thread:
  - `gh pr-review threads unresolve -R owner/repo --pr N --thread-id PRRT_...`

Notes:
- Thread URL is not exposed in GraphQL; derive navigation via the first comment’s `html_url` if needed (not included in minimal schema).
- `--mine` filter includes threads you can resolve/unresolve.

## Selector examples
- `gh pr-review comments ids agyn-sandbox/gh-pr-review-e2e-20251202#4 --review_id 3531807471`
- `gh pr-review threads find -R agyn-sandbox/gh-pr-review-e2e-20251202 --pr 4 --comment_id 2582545223`
- `gh pr review --start https://github.com/agyn-sandbox/repo/pull/123`

## Error handling
- Expect errors like "not found", "permission denied", "invalid selector".
- Ensure `gh auth status` is green and your token has required scopes for private repos.

All commands return JSON-only outputs with aligned REST/GraphQL field names and emit-only-present policy.
