# ABox Client Providers

Local capabilities exposed to cloud agents through the WebSocket tunnel.

## Implemented

| Provider | Path Prefix | Description |
|----------|------------|-------------|
| **files** | `/webdav/` | Read/write local files via WebDAV |
| **browser** | `/browser/` | Control Chrome via CDP (navigate, screenshot, click, type, evaluate, tabs) |

## Planned

### Core (Phase 3.1)

| Provider | Path Prefix | Why |
|----------|------------|-----|
| **shell** | `/shell/` | Execute local commands (git, npm, make, etc.) |
| **clipboard** | `/clipboard/` | Read/write system clipboard |
| **notifications** | `/notify/` | Send desktop notifications (task done, need approval) |
| **search** | `/search/` | Full-text search local files (ripgrep/fd) |

### Dev Tools (Phase 3.2)

| Provider | Path Prefix | Why |
|----------|------------|-----|
| **git** | `/git/` | Git operations (status, diff, commit, push) without shell |
| **docker** | `/docker/` | Control local Docker (build, run, logs) |
| **database** | `/db/` | Query local databases (SQLite, Postgres, MySQL) |
| **terminal** | `/terminal/` | Persistent terminal sessions (PTY over tunnel) |

### OS Integration (Phase 3.3)

| Provider | Path Prefix | Why |
|----------|------------|-----|
| **screenshot** | `/screen/` | Capture desktop screenshots (not just browser) |
| **apps** | `/apps/` | Launch/control native apps (AppleScript/xdotool) |
| **keychain** | `/keychain/` | Read secrets/passwords (with user approval) |
| **calendar** | `/calendar/` | Read calendar events |
| **contacts** | `/contacts/` | Read contacts |

### Data Sources (Phase 3.4)

| Provider | Path Prefix | Why |
|----------|------------|-----|
| **email** | `/email/` | Read local email (IMAP/maildir) |
| **notes** | `/notes/` | Access Apple Notes / Obsidian / Notion local |
| **photos** | `/photos/` | Access photo library (search by date/location) |
| **history** | `/history/` | Browser history + shell history |

## Security Model

Every provider requires explicit opt-in:
```bash
aboxctl client \
  --browser \          # enable browser
  --shell \            # enable shell
  --clipboard \        # enable clipboard
  --roots ~/projects   # enable files for specific dirs
```

Sensitive providers (shell, keychain, apps) require user confirmation per-action:
- Agent requests action → Client shows desktop notification
- User approves/denies → result sent back through tunnel

## Provider Interface

All providers implement `http.Handler` and register with `client.AddProvider(name, handler)`.
