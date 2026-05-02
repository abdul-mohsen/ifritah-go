# chat/

Two-way message channel between the **backend** (`ifritah-go`) and the **frontend** (`afrita-go`).

## How it works

- **Backend → Frontend** messages: written by the backend team into the **frontend** repo at `afrita-go/chat/` and pushed as a small PR.
- **Frontend → Backend** messages: written by the frontend team into the **backend** repo at `ifritah-go/chat/` and pushed as a small PR.

Each side reads the `chat/` folder in its own repo for incoming notes and writes into the other repo's `chat/` folder for outgoing notes. This keeps each message visible in normal git history (PR review, blame, etc.) and avoids any external coordination tool.

## File naming

```
chat/YYYY-MM-DD_<from>-to-<to>_<short-slug>.md
```

Examples:
- `chat/2026-05-02_frontend-to-backend_list-endpoints-400.md`
- `chat/2026-05-03_backend-to-frontend_seed-script-patched.md`

## Message template

```markdown
# <one-line title>

**Date:** YYYY-MM-DD
**From:** backend  (ifritah-go @ <branch> @ <short-sha>)
**To:**   frontend (afrita-go @ <branch>)
**Severity:** P0 | P1 | P2 | P3
**Status:** open | in-progress | resolved
**Related:** <PR / issue / run links>

## TL;DR
One paragraph.

## Reproduction
Exact commands / payloads.

## Root cause (if known)
Code references with file:line links.

## Recommended fix
Diff-style suggestion or option list.

## Blocked / impacted work
Bullet list of tests, features, or callers affected.
```

## Reply convention

When replying, create a **new file** rather than editing the original. Reference the prior message at the top:

```markdown
**Re:** chat/2026-05-02_frontend-to-backend_list-endpoints-400.md
```

This keeps history append-only and easy to audit.

## Closing a thread

When a request is fully resolved, the side that opened it appends a final note titled `*-resolved.md` and updates `Status:` to `resolved`. Both repos keep the entire thread for the record.
