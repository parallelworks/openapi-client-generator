# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`openapi-client-generator` is a Go code generator that turns an OpenAPI 3.1 (or 3.0) spec into a complete, idiomatic Go HTTP client package with zero external runtime dependencies.

The main branch is `canary`. Releases are managed by release-please (conventional-commit driven).

## Pipeline

A spec flows through three stages, one per `internal/` package:

1. **parse** (`internal/parser`) — load and validate the spec with libopenapi, resolve `$ref`s.
2. **analyze** (`internal/analyzer`) — lower the spec into the intermediate representation (`internal/ir`): operations, params, types, pagination. This is where naming, disambiguation, and pagination detection happen.
3. **generate** (`internal/generator`) — render the IR through `text/template` files (`internal/templates/*.tmpl`, embedded via `embed.go`) and post-process each file with `goimports` (`WriteFiles`).

Supporting packages: `internal/naming` (OpenAPI identifier → Go identifier), `cmd/` (the Cobra CLI).

The generated code is the product. It must always compile — the e2e tests in `internal/generator` generate clients from specs and run `go build`/`go test` on the output.

## Commands

```bash
go build ./...                 # build
go test ./...                  # run all tests (incl. generate-and-compile e2e tests)
go test ./internal/generator/  # one package
gofmt -l .                     # list unformatted files
go vet ./...

# Run the generator
go run . generate --spec testdata/petstore.yaml --out ./gen/petstore
```

Template changes don't need a build step — templates are `go:embed`ed and read at generate time.

## Code Standards

### Commits
Conventional commits; PR titles become changelog entries.
- `feat:` new feature · `fix:` bug fix · `docs:` · `chore:` build/tooling · `refactor:`
- `!` for breaking changes (`feat!:`); scope for context (`fix(generator):`).
- For `fix` titles, describe the **broken behavior** the user saw, not the code action. For `feat`, don't start with "add" — the title is the new capability.

### Comments
Default to writing no comments. Only add one when the WHY is non-obvious: a hidden constraint, a subtle invariant, a workaround for a specific bug, behavior that would surprise a reader. If removing the comment wouldn't confuse a future reader, don't write it. Specifically avoid:
- WHAT comments. Names and code already say what — `// hasRequiredQueryParams returns whether there are required query params` above a function of that name is noise.
- Conventions that hold across the package (e.g. "wire names use OrigName" is true everywhere, so don't repeat it on each helper).
- References to current task / fix / callers (`// so foo_test.go can exercise this`, `// matching how auth cookies are built`, `// added for the X flow`) — they rot. Such context belongs in the PR description.
- Multi-paragraph docstrings. One short line max — if you need more, the function is doing too much.
- Decorative section headers (`// ===== Helpers =====`). Use blank lines or split files.

Note: a generated client's exported types/methods are SDK documentation for its consumers, so a concise doc comment there is appropriate. Unexported template helpers (`addQueryParam`, etc.) are internal — apply the rules above.

### DRY
When the same logic appears in multiple places, extract a shared helper. Before writing new code, check whether an existing helper already does it. When adding encoding/naming/forwarding that mirrors existing code, reuse it rather than duplicating.

### Go
- Don't `panic`; return errors.
- Use `errors.Is`/`errors.As` for error checks, not equality, so wrapped errors match.
- Use `time.Time` for timestamps; let the JSON marshaler format them.
- File names: lowercase, no separators (`funcmap.go`) or underscores (`api_key.go`) — no camelCase or hyphens. Test files end `_test.go`.
- Package directories under `internal/`: lowercase, no separators.
- All code must pass `gofmt` and `go vet`.

### Testing
- **Test real code.** Tests import and call the actual functions under test — never reimplement the logic being tested. For the generator, the strongest tests generate a client from a spec and then compile/run it (`internal/generator/e2e_test.go`), so a broken template fails the test rather than a restated assertion.
- Prefer `t.Context()` over `context.Background()` in tests.
- When changing a template, add or extend an e2e test that compiles the affected operation shape.
