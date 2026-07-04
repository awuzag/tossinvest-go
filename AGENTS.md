# AGENTS.md

This repository is an unofficial Go SDK and CLI for Toss Invest OpenAPI.

## Rules

- Do not imply this is an official Toss Securities or Toss Invest project.
- Do not commit credentials, tokens, account numbers, or local `.env` files.
- Keep prices, amounts, and quantities as strings unless a caller explicitly chooses a decimal package.
- Default tests must not call live Toss Invest endpoints.
- Live order create, modify, or cancel checks must require explicit opt-in and must not run in ordinary `go test ./...`. The CLI uses `enable` commands rather than activation flags.
- Run `gofmt` after Go edits.
- Run `go test ./...` and `go test ./... -cover` before reporting completion.
