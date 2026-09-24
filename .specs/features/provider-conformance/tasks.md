# Native provider conformance tasks

**Design:** `.specs/features/provider-conformance/design.md`  
**Status:** In progress

`T1 -> T2 -> T3 -> T4`

## T1: Add fixture harness and buffered scenarios — Complete

**What:** Load external JSON fixtures and verify translated requests,
authentication shape, buffered responses, media, tools, and upstream errors.
**Where:** `internal/provider/conformance_test.go`,
`internal/provider/testdata/conformance/`.
**Depends on:** None. **Requirements:** PC-01, PC-02.
**Tests:** Integration with `httptest`. **Gate:** Full.

## T2: Add streaming conformance scenarios

**What:** Cover normalized text/tool SSE for Anthropic and Google protocols,
and Bedrock's explicit pre-network streaming rejection.
**Where:** The fixture harness and conformance fixture files.
**Depends on:** T1. **Requirement:** PC-02.
**Tests:** Integration with `httptest`. **Gate:** Full.

## T3: Add the opt-in live runner

**What:** Add build-tagged, environment-driven text, media, and tool smoke
scenarios with explicit credentials and token limits.
**Where:** `tests/provider-live/`.
**Depends on:** T2. **Requirement:** PC-03.
**Tests:** Compile by default; execute only with the `live` build tag.
**Gate:** Full.

## T4: Publish the validation matrix

**What:** Document commands, environment variables, evidence semantics,
current coverage, and the next validation work; update project state.
**Where:** `docs/provider-validation.md`, `docs/providers.md`,
`.specs/project/STATE.md`, and this feature specification.
**Depends on:** T3. **Requirement:** PC-04.
**Tests:** Race, vet, build, and diff check. **Gate:** Build.
