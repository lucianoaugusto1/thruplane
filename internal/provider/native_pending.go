package provider

import (
	"context"
	"net/http"

	"nexoroute/internal/config"
)

type pendingAdapter struct{ provider string }

func (a pendingAdapter) buildRequest(context.Context, []byte, string) (*http.Request, error) {
	return nil, &RequestError{Code: "unsupported_provider_feature", Message: a.provider + " adapter is not available in this build."}
}

func (pendingAdapter) normalizeResponse(response *http.Response, _ string, _ bool) (*http.Response, error) {
	return response, nil
}

func newBedrockAdapter(config.ProviderConfig) (adapter, error) {
	return pendingAdapter{provider: "Amazon Bedrock"}, nil
}
