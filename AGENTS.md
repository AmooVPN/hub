# AGENTS.md

## Scope
- This repository implements `hub`, the centralized multi-panel management hub described in `TODO.md`.
- The current build priority is the initial milestone: config loading, SQLite, Redis, migrations, encryption, password hashing, models, repositories, and startup wiring.

## Product Rules
- Never directly manipulate a 3x-ui database.
- Use SQLite as the primary database.
- Use Redis for cache and distributed locking related work.
- Keep secrets out of logs and responses.
- Prefer server-side rendering and HTMX-compatible pages when UI work is added.

## Engineering Rules
- Keep changes minimal and targeted.
- Prefer the smallest correct implementation.
- Preserve the existing layout and naming unless a change is explicitly required.
- Add tests for security-sensitive or data-layer changes when practical.
- Use ASCII only unless existing files require otherwise.

## Startup Expectations
- Load and validate configuration first.
- Create the data and backup directories.
- Open SQLite and apply pragmas.
- Connect Redis and fail startup if it is unavailable.
- Run migrations before serving traffic.
- Bootstrap the initial admin from environment variables when the database is empty.
