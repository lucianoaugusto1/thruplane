# Native multimodal chat tasks

**Design**: `.specs/features/native-multimodal/design.md`
**Status**: In progress

`T1 -> {T2, T3, T4} -> T5 -> T6`

### T1: Parse portable content parts

**What**: Validate and preserve ordered text, image, PDF, and audio parts.
**Where**: `internal/provider/native_content.go` and co-located tests.
**Depends on**: None.
**Requirement**: MM-01.
**Tests**: Unit; quick gate.

### T2: Translate Anthropic media

**What**: Map images and PDFs into Messages content blocks.
**Where**: `internal/provider/anthropic.go` and co-located tests.
**Depends on**: T1.
**Requirement**: MM-02.
**Tests**: Integration with `httptest`; full gate.

### T3: Translate Google media

**What**: Map inline images, PDFs, and audio into Gemini and Vertex parts.
**Where**: `internal/provider/google.go` and co-located tests.
**Depends on**: T1.
**Requirement**: MM-03.
**Tests**: Integration with `httptest`; full gate.

### T4: Translate Bedrock media

**What**: Map inline images and PDFs into Converse blocks.
**Where**: `internal/provider/bedrock.go` and co-located tests.
**Depends on**: T1.
**Requirement**: MM-04.
**Tests**: Integration with `httptest`; full gate.

### T5: Align routing and discovery

**What**: Advertise newly translated input modalities and retain rejections.
**Where**: `internal/provider/capabilities.go`, gateway tests.
**Depends on**: T2, T3, T4.
**Requirement**: MM-05.
**Tests**: Integration with `httptest`; full gate.

### T6: Documentation and local benchmarks

**What**: Document exact media shapes and benchmark translation overhead.
**Where**: `docs/providers.md`, `docs/model-catalog.md`, README, benchmark.
**Depends on**: T5.
**Requirement**: MM-06.
**Tests**: Build gate and benchmark run.

## Validation

| Check | Result |
| --- | --- |
| Task granularity | Each task has one adapter or one contract deliverable. |
| Dependencies | T1 precedes adapters; all adapters precede capability claims. |
| Test co-location | Parser has unit tests; adapters and routing use `httptest`. |
