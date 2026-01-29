# AI Continuity & Handoff Rules

You are an AI assistant continuing work on LocalMesh. This file ensures you maintain context across sessions.

## Session Start Protocol

When user mentions "continue LocalMesh", "pick up where we left off", or "read the handoff":

1. **Read `/AI_HANDOFF.md`** - Current state, mistakes to avoid, architecture
2. **Read `/PLAN.md`** - Roadmap with detailed ASCII diagrams
3. **Check `/aiSkills/*.md`** - All coding rules and patterns
4. **Ask user** - "What would you like to work on from the roadmap?"

## Documentation Update Protocol

After completing significant work:

1. **Update `AI_HANDOFF.md`** with:
   - New features completed (move from "Planned" to "Implemented")
   - New mistakes discovered
   - Architecture changes

2. **Update `PLAN.md`** with:
   - Checked items `[x]` in implementation checklists
   - New phases if scope changes

3. **Git commit** the documentation updates

## Code Writing Protocol

Before writing ANY code:

1. **Read the relevant aiSkill file:**
   - Go code → `go-localmesh.md` + `security-first.md`
   - Security-sensitive → `security-first.md` (REQUIRED)
   - Scalability concerns → `go-backend-scalability.md`

2. **Check existing patterns** in the codebase:
   - Similar files in the same package
   - How other handlers/services are structured
   - Existing error handling patterns

3. **Follow the rules** - No exceptions for:
   - Parameterized SQL queries
   - Error wrapping with context
   - PASETO over JWT
   - crypto/rand over math/rand
   - slog for logging

## Quality Assurance

Before suggesting any code is complete:

1. **Mental lint check:**
   - Would `golangci-lint` pass?
   - Are all errors handled with context?
   - Are there any SQL injection risks?

2. **Suggest testing:**
   - `make build` should succeed
   - `go test ./...` should pass
   - Manual testing with `sudo ./localmesh start --dev`

## Communication Style

The user prefers:
- Detailed explanations of WHY decisions are made
- ASCII diagrams for architecture
- Atomic git commits with conventional format
- Learning alongside doing

When explaining:
- Don't just show code, explain the reasoning
- Connect to broader architecture
- Mention trade-offs

## Key Technical Decisions (Don't Reverse These)

These decisions were made after debugging and shouldn't be changed without good reason:

| Decision | Reason |
|----------|--------|
| Use `avahi-publish-address` for mDNS hostnames | zeroconf only does services, not A records |
| DNS server binds to WiFi IP, not 0.0.0.0 | Avoid conflict with systemd-resolved |
| Default hostname is "campus" not "mesh" | Avoid collision with _mesh._tcp service |
| PASETO v4 for tokens | No algorithm confusion attacks like JWT |
| SQLite + Badger for storage | Embedded, no external dependencies |
| Bubble Tea for TUI | Elm architecture, composable with Bubbles |

## Current State Markers

Update these when completing work:

```
Last Working Session: January 30, 2026
Current Phase: Phase 1 - Dynamic mDNS Hostname Assignment (70% complete)
Last Feature Completed: CLI commands (register, unregister, services, network interfaces)
Next Feature: TUI service registration form OR Phase 2.1 agent binary
Blocking Issues: None
```

### Recently Added Files (Jan 30, 2026):
- `internal/network/interfaces.go` - Network interface detection
- `internal/registry/mdns_registry.go` - mDNS service registration with avahi
- Updated `cmd/localmesh/cmd/root.go` - Added register, unregister, services commands

## Recovery From Confusion

If you're unsure about something:

1. **Search the codebase** - Use semantic_search or grep_search
2. **Read the LEARNING.md** - It's an 8-10 hour comprehensive guide
3. **Check git history** - `git log --oneline` shows what was done
4. **Ask the user** - They know the vision!

Never guess about:
- Security implementations
- Network protocol decisions  
- Architecture patterns

Always verify by reading existing code first.
