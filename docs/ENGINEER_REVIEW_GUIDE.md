# Engineer Review Guide

## Quick PR overview with gh
- View details: `gh pr view -R owner/repo --pr N`
- View with top-level comments: `gh pr view -R owner/repo --pr N --comments`
- Inspect changes: `gh pr diff -R owner/repo --pr N` and `gh pr diff -R owner/repo --pr N --name-only`

## See inline review comments
- Latest review by reviewer: `gh pr-review comments ids -R owner/repo --pr N --latest --reviewer reviewer_login`
- Specific review ID: `gh pr-review comments ids -R owner/repo --pr N --review_id 3531807471`
- Output includes `id`, `body`, `user`, timestamps, `html_url`, and `path/line` when available
- Pagination flags: `--per_page`, `--page`, `--limit`

### Get latest review ID
- Latest by reviewer: `gh pr-review review latest-id -R owner/repo --pr N --reviewer reviewer_login`
- Output fields: `{ id, user{login,id}, submitted_at, state, author_association, html_url }`
- Tip: Pass your own login to fetch the most recent review you submitted on that PR

### List comments by review ID
- `gh pr-review comments ids owner/repo#N --review_id <id>`
- Pagination: `--per_page`, `--page`, `--limit`
- Output repeats `id`, `body`, `user`, timestamps, `html_url`, `path`, `line` when present
- After retrieving the desired `id`, feed it into `gh pr-review comments reply ...`

## Reply to an inline comment
1. List comments with one of the commands above and capture the desired `id`
2. Reply: `gh pr-review comments reply -R owner/repo --pr N --comment-id 2582545223 --body "Acknowledged; will update."`

## Selector and auth notes
- Prefer `-R owner/repo --pr N`; PR URLs and `owner/repo#N` also work
- Ensure `gh auth status` reports the correct host and scopes before working on private repositories
- `gh pr-review` commands return JSON by default; no `--json` flag is needed

## Error handling
- Typical failures: `not found`, `permission denied`, `invalid selector`
- Resolve by verifying the selector, ensuring access rights, and confirming authentication
