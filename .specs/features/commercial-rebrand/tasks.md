# Commercial rebrand tasks

**Spec:** `.specs/features/commercial-rebrand/spec.md`
**Status:** In Progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5
```

### T1: Define the product identity ✅

**What:** Document the working brand, positioning, voice, and naming system.
**Where:** `docs/brand.md`
**Depends on:** None
**Requirements:** BR-01, BR-06
**Tests:** Documentation review
**Gate:** Build

**Done when:**

- [x] Tagline, one-line description, audiences, and edition names are explicit.
- [x] Provisional-name and legal-clearance status are unambiguous.

### T2: Rename the executable and package identity ✅

**What:** Rename the Go module, imports, command, binary, image, example key,
and model ownership metadata.
**Where:** `go.mod`, Go sources and tests, `cmd/nexoroute/`, `Dockerfile`,
`config.example.yaml`, `.gitignore`, `.dockerignore`
**Depends on:** T1
**Requirements:** BR-01, BR-02
**Tests:** Existing unit and integration tests
**Gate:** Build

**Done when:**

- [x] `go test -race ./...`, `go vet ./...`, and `go build ./cmd/nexoroute` pass.
- [x] Docker builds an image containing the `/nexoroute` binary.
- [x] Existing test count does not decrease.

### T3: Establish the open-source and commercial story ✅

**What:** Rewrite onboarding and add licensing, editions, contribution, and
security documentation.
**Where:** `README.md`, `LICENSE`, `docs/editions.md`, `CONTRIBUTING.md`,
`SECURITY.md`
**Depends on:** T2
**Requirements:** BR-03, BR-04, BR-05, BR-06
**Tests:** Documentation and link review
**Gate:** Build

**Done when:**

- [x] Community capabilities and current limitations match the code.
- [x] Pro and Enterprise capabilities are labeled as planned.
- [x] Apache 2.0, contribution, and private security reporting paths are clear.

### T4: Update project memory and validate the rebrand

**What:** Update project specs and record the rebrand validation.
**Where:** `.specs/`
**Depends on:** T3
**Requirements:** BR-01 through BR-06
**Tests:** Full repository search and build gate
**Gate:** Build

**Done when:**

- [ ] No stale identity remains outside historical context.
- [ ] Requirements BR-01 through BR-06 are verified.

### T5: Move and verify the repository

**What:** Move the complete Git repository to the requested home-directory path
and rerun the final gate there.
**Where:** `/Users/lucianobr01/go-llm-gateway`
**Depends on:** T4
**Requirements:** BR-07
**Tests:** Full build gate and Git status
**Gate:** Build

**Done when:**

- [ ] Destination contains the repository and complete Git history.
- [ ] Build gate passes from the destination.
- [ ] Source path no longer contains the repository.
