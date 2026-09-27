# Native Chat Completions contract tasks

**Design:** `.specs/features/native-contract/design.md`  
**Status:** Complete

`T1 -> T2 -> T3 -> T4`

## T1: Validate root, message, and tool fields

**What:** Reject unknown fields and non-function assistant tool calls in the
shared native decoder; permit only documented equivalent defaults.
**Where:** `internal/provider/native.go`, co-located tests.
**Depends on:** None. **Requirement:** NC-01.
**Tests:** Unit and `httptest`. **Gate:** Full.

## T2: Validate content-part fields

**What:** Reject unknown user/text fields and ignored `filename` or image
detail without changing valid media translations.
**Where:** `internal/provider/native_content.go`, `internal/provider/native.go`,
co-located tests and affected media fixtures.
**Depends on:** T1. **Requirement:** NC-02.
**Tests:** Unit and `httptest`. **Gate:** Full.

## T3: Verify gateway behavior

**What:** Test unknown-model native rejection and compatible passthrough.
**Where:** `internal/gateway/` tests.
**Depends on:** T2. **Requirement:** NC-03.
**Tests:** End-to-end `httptest`. **Gate:** Full.

## T4: Publish and validate the contract

**What:** Update provider docs, README, project state, and traceability.
**Where:** `docs/`, `README.md`, `.specs/`.
**Depends on:** T3. **Requirement:** NC-04.
**Tests:** Race, vet, build, diff check. **Gate:** Build.

## Verification

- `go test ./...` and `go test -race ./...` passed on September 22, 2026.
- `go vet ./...`, `go build -o /dev/null ./cmd/thruplane`, and
  `git diff --check` passed.
- Public gateway tests cover uncataloged native rejection before network I/O
  and compatible passthrough. Live provider conformance is a separate task.
