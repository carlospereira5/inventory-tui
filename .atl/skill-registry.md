# Skill Registry for inventory-tui
## Project: inventory-tui

**Detected Stack**: Go 1.25.0, Bubbletea TUI, SQLite, Loyverse API integration


## Relevant Skills

### Go Development
- **go-testing** - Go testing patterns including Bubbletea TUI testing
- **go-error-handling** - Go error handling patterns
- **go-concurrency** - Concurrent Go code (goroutines, channels, mutexes)
- **go-context** - context.Context patterns
- **go-data-structures** - Go slices, maps, arrays
- **go-packages** - Go package organization
- **go-naming** - Go naming conventions
- **go-functions** - Go function design
- **go-declarations** - Go variable/constant declarations
- **go-interfaces** - Go interface design
- **go-style-core** - Core Go style principles
- **go-logging** - Go logging approaches
- **go-linting** - Go linting configuration
- **go-performance** - Go performance optimization
- **go-code-review** - Go code review standards
- **go-defensive** - Defensive Go programming
- **go-generics** - Go generics usage
- **go-control-flow** - Go control flow patterns
- **go-functional-options** - Go functional options pattern

### TUI Development
- **building-glamorous-tuis** - Terminal UIs with Charmbracelet
- **charm-stack** - Bubbletea, Bubbles, Lipgloss, Huh
- **gentleman-bubbletea** - Bubbletea TUI patterns for installer

### SDD Workflow
- **sdd-init** - Initialize SDD context
- **sdd-explore** - Explore ideas before committing to changes
- **sdd-propose** - Create change proposals
- **sdd-spec** - Write specifications
- **sdd-design** - Create technical design
- **sdd-tasks** - Break down changes into tasks
- **sdd-apply** - Implement tasks
- **sdd-verify** - Validate implementation
- **sdd-archive** - Archive completed changes

### Other
- **skill-creator** - Create new AI agent skills
- **find-skills** - Discover and install agent skills
- **deploy-to-vercel** - Deploy to Vercel

## Project Conventions

No project-level agent files found (no AGENTS.md, CLAUDE.md, .cursorrules, etc.)

## Architecture Detected

- **Domain-Driven Design**: Clear separation (domain/, application/, infrastructure/)
- **SQLite Database**: Using modernc.org/sqlite with WAL mode
- **Loyverse Integration**: Webhooks and event handling for POS sync
- **Dual Interface**: TUI (cmd/tui/) and Webapp (cmd/webapp/) versions
- **Structured Logging**: Loyverse logging infrastructure

## Triggers

When working on:
- Go tests → load `go-testing`
- Bubbletea TUI → load `building-glamorous-tuis`, `charm-stack`, `gentleman-bubbletea`
- Error handling → load `go-error-handling`
- Concurrency → load `go-concurrency`
- SDD workflow → load appropriate `sdd-*` skill
