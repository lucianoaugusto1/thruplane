# Model catalog

NexoRoute ships a versioned model catalog under `internal/catalog/data`. The
catalog records facts that the gateway needs for capability-safe routing and
model discovery. The files are embedded in the executable and parsed once at
startup, so they don't add file I/O or YAML parsing to the request path.

`GET /v1/models/{alias}` returns each configured target's catalog identity,
limits, pricing, performance, source URLs, and two capability sets:
`catalog_capabilities` describes the provider model and
`effective_capabilities` shows what the current adapter implements. The public
gateway endpoint is Chat Completions only; an operation such as `responses` in
the catalog is not a gateway endpoint.

The catalog is curated, not exhaustive. Providers release, rename, and retire
models independently of NexoRoute releases. Use the provider's live model API
or console to confirm availability in your account and region.

## Recorded fields

Each provider file records:

- The retrieval date and official source URLs.
- Model IDs, aliases, families, lifecycle status, and knowledge cutoffs.
- Supported operations and input and output modalities.
- Streaming, structured output, reasoning, prompt caching, and function-tool
  support.
- Context, input, output, image, and document limits when published.
- Prices with currency, billing unit, modality, and qualifying condition.
- Qualitative latency and optional measured time to first token (TTFT) and
  output tokens per second (TPS).

Prices don't use a fixed input/output structure. A model can have multiple
rates for cache reads, cache writes, cache storage, long context, audio, images,
or another provider-specific billing dimension. Every rate includes its unit
and any condition that selects it.

## Performance data

Provider-independent latency and TPS numbers aren't available for most hosted
models. They change with region, service tier, prompt size, output size,
reasoning effort, concurrency, and provider load. Ollama additionally depends
on local hardware, quantization, and runtime settings.

NexoRoute leaves `time_to_first_token_ms` and
`output_tokens_per_second` unset unless a value has a reproducible measurement
method. It never converts marketing terms such as "fast" into fabricated
numbers. `latency_class` preserves an official qualitative comparison when one
exists.

## Provider coverage

- OpenAI: Current flagship GPT models with short- and long-context prices,
  cached input, and cache writes.
- Anthropic: Current Claude lineup with five-minute and one-hour cache writes,
  cache reads, limits, and comparative latency.
- Gemini Developer API: Current general-purpose Gemini models with multimodal
  input and cache storage rates.
- Vertex AI: Gemini model capabilities and deployment-sensitive pricing notes.
- Amazon Bedrock: Amazon Nova models supported by the Converse adapter. Bedrock
  has a much larger regional catalog, so other model IDs require an explicit
  catalog mapping when strict mode is enabled.
- Azure OpenAI: Current OpenAI model capabilities. Azure prices remain
  deployment-specific because region and service tier affect the rate.
- Ollama: Common local model families. Cost and performance must be measured on
  the customer's hardware and selected tag.
- xAI: Current Grok chat models, cache rates, and long-context pricing.
- OpenAI-compatible: No built-in model assumptions. The endpoint owner defines
  its models and semantics.
- NexoRoute Inference: No built-in model entries until the hosted service has a
  published, versioned model contract.

## Routing policy

NexoRoute inspects Chat Completions requests for input and output modalities,
function tools, strict schemas, parallel calls, structured output, and
streaming. Known models are eligible only if both the catalog entry and the
adapter support every required feature. Native adapters currently translate
text and function tools, not image, audio, video, or document parts. Bedrock
streaming and portable strict tool schemas are also unavailable. Incompatible
targets are skipped before any upstream call; if none remain, the gateway
returns HTTP 400 with `unsupported_capability`.

Unknown model IDs pass through by default to preserve private endpoints and
new provider releases. This mode cannot guarantee capability-safe routing for
unknown IDs. Set `catalog.unknown_models: reject` for a closed catalog; if all
targets are unknown, the gateway returns `model_not_cataloged`.

Deployment identifiers can map to a catalog entry without changing the
upstream request:

```yaml
models:
  production:
    targets:
      - provider: azure
        model: production-deployment
        catalog_model: gpt-4o-mini
```

The optional `catalog_provider` field overrides the provider namespace for
lookup. Use it only when the target API actually serves that model contract.
Catalog pricing is reference metadata, not a live bill or a cost router.
Regional, tier, and contract prices may differ. OpenAI's published
GPT-5.6 Sol prices are promotional through at least November 21, 2026.

## Primary sources

- [OpenAI models](https://developers.openai.com/api/docs/models)
- [OpenAI model comparison](https://developers.openai.com/api/docs/models/compare)
- [Claude models overview](https://platform.claude.com/docs/en/models/overview)
- [Claude API pricing](https://platform.claude.com/docs/en/about-claude/pricing)
- [Gemini models](https://ai.google.dev/gemini-api/docs/models)
- [Gemini API pricing](https://ai.google.dev/gemini-api/docs/pricing)
- [Vertex AI generative AI pricing](https://cloud.google.com/gemini-enterprise-agent-platform/generative-ai/pricing)
- [Amazon Bedrock models](https://docs.aws.amazon.com/bedrock/latest/userguide/models.html)
- [Amazon Bedrock pricing](https://aws.amazon.com/bedrock/pricing/)
- [Azure OpenAI models](https://learn.microsoft.com/en-us/azure/ai-services/openai/concepts/models)
- [Ollama model library](https://ollama.com/library)
- [xAI models](https://docs.x.ai/developers/models)
- [xAI API pricing](https://docs.x.ai/developers/pricing)

## Updating the catalog

When you change a provider file:

1. Use only first-party documentation.
2. Update `provider.updated_at`.
3. Preserve the exact billing unit and price condition.
4. Leave unpublished values empty.
5. Run `go test ./internal/catalog` to validate every embedded file.
