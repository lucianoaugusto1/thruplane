package provider

import (
	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
)

// EffectiveCapabilities intersects model facts with features implemented by
// this gateway's adapter. Catalog facts alone do not imply transport support.
func EffectiveCapabilities(provider config.ProviderConfig, model catalog.Model) catalog.Capabilities {
	caps := model.Capabilities
	caps.Operations = intersect(caps.Operations, []string{"chat"})
	switch provider.Type {
	case "anthropic", "bedrock":
		caps.InputModalities = intersect(caps.InputModalities, []string{"text", "image", "document"})
		caps.OutputModalities = intersect(caps.OutputModalities, []string{"text"})
		caps.StructuredOutputs = false
		caps.Tools.StrictSchema = false
		caps.PromptCaching = false
		if provider.Type == "bedrock" {
			caps.Streaming = false
		}
	case "gemini", "vertex":
		caps.InputModalities = intersect(caps.InputModalities, []string{"text", "image", "audio", "document"})
		caps.OutputModalities = intersect(caps.OutputModalities, []string{"text"})
		caps.StructuredOutputs = false
		caps.Tools.StrictSchema = false
		caps.PromptCaching = false
	case "openai", "azure-openai", "xai", "ollama", "openai-compatible", "nexoroute-inference", "":
		// These adapters pass the OpenAI-compatible request through unchanged.
	}
	return caps
}

func SupportsRequirements(provider config.ProviderConfig, requirements catalog.Requirements) bool {
	if requirements.RemoteImage {
		switch provider.Type {
		case "gemini", "vertex", "bedrock", "ollama":
			return false
		}
	}
	if requirements.DisableParallelTools && requirements.Tools {
		return provider.Type != "gemini" && provider.Type != "vertex" && provider.Type != "bedrock"
	}
	return true
}

func intersect(values, allowed []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, candidate := range allowed {
			if value == candidate {
				result = append(result, value)
				break
			}
		}
	}
	return result
}
