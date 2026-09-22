# Model capability catalog tasks

**Design**: `.specs/features/model-capability-catalog/design.md`
**Status**: Complete

## Execution plan

`T1 -> T2 -> T3 -> T4 -> T5`

### T1: Define and validate the catalog

**What**: Add catalog types, strict embedded loading, lookup, and unit tests.
**Where**: `internal/catalog/`
**Depends on**: None
**Requirements**: CAT-01, CAT-02
**Tests**: Unit
**Gate**: Quick

### T2: Add sourced provider catalogs

**What**: Add versioned YAML data and source documentation for initial providers.
**Where**: `internal/catalog/data/`, `docs/model-catalog.md`
**Depends on**: T1
**Requirements**: CAT-02
**Tests**: Catalog validation unit tests
**Gate**: Quick

### T3: Add configuration and request requirement detection

**What**: Add unknown-model policy, catalog mappings, and request inspection.
**Where**: `internal/config/`, `internal/catalog/request.go`
**Depends on**: T1
**Requirements**: CAT-03, CAT-06
**Tests**: Unit
**Gate**: Quick

### T4: Integrate capability-aware routing

**What**: Filter targets before upstream calls and return explicit errors.
**Where**: `internal/gateway/`
**Depends on**: T2, T3
**Requirements**: CAT-03, CAT-04
**Tests**: Integration with `httptest`
**Gate**: Full

### T5: Expose metadata and complete documentation

**What**: Extend model details and document configuration and data semantics.
**Where**: `internal/gateway/models.go`, configuration examples, documentation
**Depends on**: T4
**Requirements**: CAT-05
**Tests**: Endpoint tests
**Gate**: Build

## Validation

| Check | Result |
| --- | --- |
| Task granularity | Each task has one cohesive deliverable. |
| Dependency diagram | Every dependency appears in the sequential diagram. |
| Test co-location | Code tasks include the test type required by `TESTING.md`. |
