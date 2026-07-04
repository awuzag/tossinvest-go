# Contract-First Code Generation Plan

## Baseline Problem

The first SDK spike was useful as a working baseline, but the public surface was mostly hand-written:

- `api.go` manually maps 21 operations to HTTP paths, query parameters, account headers, and response envelopes.
- `types.go` manually defined only a small subset of request/response models and used `RawObject` aliases for many result types.
- `internal/cli/cli.go` manually repeats command names, flags, and operation mapping.
- Tests prove the current mapping, but they do not prevent future OpenAPI changes from silently drifting away from the code.

That was acceptable for the first spike, but not for long-term provider maintenance. Toss Invest OpenAPI is now treated as the core contract, and generated code owns provider-native request/response shapes and raw API execution.

## Contract Source

Canonical contract:

- `contracts/tossinvest/openapi.json`
- upstream: `https://openapi.tossinvest.com/openapi-docs/latest/openapi.json`
- current snapshot: OpenAPI `3.1.0`, API version `1.0.3`, 21 operations, 53 schemas

The repo should preserve the canonical spec as a reviewed artifact. Code generation should read only from this file plus explicit local overrides.

## Target Layering

```text
contracts/tossinvest/openapi.json
    |
    v
internal/codegen/tossopenapi
    |
    +--> internal/generated/tossapi/types_gen.go
    +--> internal/generated/tossapi/client_gen.go
    +--> internal/generated/tossapi/catalog_gen.go
    |
    v
generated runtime adapter
    |
    v
public middle layer: package tossinvest
    |
    +--> stable SDK methods
    +--> auth/token cache
    +--> account selection helpers
    +--> error wrapping
    +--> rate-limit handling
    |
    v
CLI and future mwosa provider adapter
```

The generated layer should be provider-native and mechanical. The public `tossinvest` package should be the middle layer that gives callers a stable, ergonomic API while delegating raw request/response types to generated code.

## Generated Layer Responsibilities

Generated code should own:

- operation specs: method, path, operationId, tag, summary
- request parameter structs
- request body structs
- response model structs
- success envelope result types
- API catalog metadata for CLI wiring
- low-level typed operation methods such as `GetPrices`, `CreateOrder`, `GetAccounts`

Generated code should not own:

- OAuth token lifecycle policy
- retry/rate-limit policy
- live trading safety gates
- CLI UX wording
- mwosa role mapping
- business-friendly method names when they intentionally differ from operationId

## Middle Layer Responsibilities

The hand-written middle layer should own:

- `New(...)` options and HTTP client configuration
- `Token` issuance and token storage
- authorization and `X-Tossinvest-Account` injection
- common error types: HTTP, OAuth, Toss API envelope, decode
- optional account selection defaults
- stable convenience methods such as `Prices(ctx, []string)`
- account/order/live-trading opt-in guardrails in SDK and CLI
- future mwosa provider role adapter boundaries

This keeps generated churn away from application-facing code. When Toss changes the spec, generated files can change heavily while the middle layer absorbs or intentionally exposes the change.

## Generator Options

### Option A: custom repo-local generator

This follows the existing awuzag pattern most closely.

Relevant references:

- `kis/internal/codegen/kisopenapi`
- `kis/internal/generated/rawapi`
- `opendart/internal/generated/opendartapi`
- `opendart/internal/cli/catalog_gen.go`

Pros:

- Full control over Toss-specific envelopes, OAuth exception, account header, decimal string handling, and CLI metadata.
- Works around OpenAPI 3.1 generator gaps without waiting on upstream tools.
- Can generate exactly the small raw layer this repo wants.
- Easier to preserve awuzag naming and provider role conventions.

Cons:

- More code to maintain locally.
- Must test generator output carefully.

### Option B: `oapi-codegen`

`oapi-codegen` is a Go OpenAPI generator for types, clients, and server code. It is a strong candidate if it can handle the Toss OpenAPI 3.1 snapshot after either direct validation or a controlled 3.1-to-3.0 conversion.

Pros:

- Mature Go ecosystem fit.
- Good for generating model types and client request boilerplate.
- Less custom generator code.

Risks:

- OpenAPI 3.1 support and `oneOf` behavior must be tested against `contracts/tossinvest/openapi.json`.
- Generated client shape may not match the middle-layer boundary we want.
- Toss envelope specialization may still require wrappers.

### Option C: `ogen`

`ogen` generates typed Go clients/servers from OpenAPI v3 and is another strong candidate.

Pros:

- Strongly typed client generation.
- Runtime validation may be useful for contract drift.

Risks:

- Must verify OpenAPI 3.1 compatibility and Toss schema features.
- Generated public shape may be larger than desired.

## Recommended Direction

Start with a custom repo-local generator, then keep `oapi-codegen`/`ogen` as evaluation tracks.

Reasoning:

- Toss OpenAPI is small: 21 operations and 53 schemas.
- The API has non-uniform behavior: OAuth token endpoint uses OAuth format, ordinary APIs use `result`/`error` envelopes, account-scoped APIs require a custom header, and order APIs need safety gates.
- Existing awuzag repos already use repo-local generated raw layers plus hand-written public layers.
- The desired architecture is not just "generate a client"; it is "generate provider-native contract code under a stable middle layer."

## Proposed File Layout

```text
contracts/tossinvest/openapi.json
contracts/tossinvest/README.md
docs/apis/README.md
internal/codegen/tossopenapi/main.go
internal/codegen/tossopenapi/main_test.go
internal/generated/tossapi/types_gen.go
internal/generated/tossapi/client_gen.go
internal/generated/tossapi/catalog_gen.go
generated.go
api.go
types.go
internal/cli/
```

`generated.go` should adapt the generated runtime to the public `Client`, similar in spirit to `opendart/generated_api.go`.

## Migration Steps

1. Add `//go:generate go run ./internal/codegen/tossopenapi` in a small root file.
2. Build a generator that parses `contracts/tossinvest/openapi.json` and emits operation catalog metadata first.
3. Generate request parameter structs and response type structs.
4. Generate raw operation methods against a small `Executor` interface.
5. Change hand-written public methods in `api.go` to call generated raw methods.
6. Replace `RawObject` response aliases with generated types where practical.
7. Generate CLI catalog metadata, then keep CLI command execution hand-written.
8. Add a contract drift test that compares generated catalog operation count/IDs against `contracts/tossinvest/openapi.json`.
9. Add a regeneration check in `task verify`.

## Contract Drift Gates

Add tests or scripts that fail when:

- `contracts/tossinvest/openapi.json` operation count differs from generated catalog count.
- an operationId exists in the spec but not in generated catalog.
- generated files are stale after `go generate ./...`.
- account-scoped APIs are missing `X-Tossinvest-Account` metadata.
- account/order/live-trading APIs are exposed without opt-in safety metadata.

## Non-Goals

- Do not generate mwosa provider adapters yet.
- Do not expose generated raw packages as the primary public API.
- Do not remove the account, order, or live-trading guards.
- Do not use float types for decimal values.
- Do not make OpenAPI refresh automatic without review; the checked-in spec remains the contract.

## Immediate Next Step

Implement generation in this order:

1. `internal/generated/tossapi/catalog_gen.go`
2. drift test for operation IDs
3. generated request structs
4. generated response structs
5. generated raw client methods
6. public middle-layer wrappers

This gets contract enforcement early, before replacing the entire current SDK.
