# Architecture refactor design

**Specification:** `spec.md`
**Status:** Approved

## Approach

Keep the current packages and move responsibilities behind unexported,
integration-specific types. This preserves short dependency distance in Go
while reducing the number of reasons each file changes.

```text
HTTP handler -> request planner -> upstream executor -> HTTP relay
                     |                    |
                     +-> model catalog    +-> provider client and limiter
```

## Components

### Gateway settings

An unexported immutable snapshot retains the request body limit, catalog
policy, configured routes, provider types, and retry settings. Constructors
continue to accept `config.Config`, so callers do not change.

### Chat planner

The planner validates the public request, inspects required capabilities, and
returns the ordered routable targets. It returns a structured public error
instead of writing an HTTP response.

### Upstream executor

The executor owns admission, retry budgets, provider calls, response
classification, and fallback. It returns a response or a structured execution
failure. It does not write to `http.ResponseWriter`.

### HTTP transport

The handler and transport helpers read bounded request bodies, render public
errors, relay headers, and stream response bodies.

### Native provider contract

The existing provider package remains a single package. Native wire types,
request helpers, tool validation, and normalized responses move to focused
files. This avoids exporting internal types merely to create subpackages.

### Configuration

The configuration package remains one package. Types, loading, defaults and
normalization, and validation move to focused files. The YAML schema and
validation messages remain unchanged.

## Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Package structure | Keep existing packages | Avoid new exported internals and package noise. |
| Public constructors | Preserve signatures | Avoid caller and test churn. |
| Behavior | Mechanical extraction | Keep the refactor independently reviewable. |
| Dependencies | Add none | Preserve the small, standard-library-oriented core. |

## Error handling

Planning and execution return internal structured errors. The HTTP boundary
maps those errors to the existing OpenAI-shaped error response. Request
cancellation remains silent after the context ends, matching current behavior.
