package gateway

import "github.com/lucianoaugusto1/thruplane/internal/config"

type gatewaySettings struct {
	maxBodyBytes  int64
	unknownModels string
	providerTypes map[string]string
	models        map[string]config.ModelConfig
	routing       config.RoutingConfig
}

func newGatewaySettings(cfg config.Config) gatewaySettings {
	providerTypes := make(map[string]string, len(cfg.Providers))
	for name, provider := range cfg.Providers {
		providerTypes[name] = provider.Type
	}

	models := make(map[string]config.ModelConfig, len(cfg.Models))
	for alias, model := range cfg.Models {
		model.Targets = append([]config.TargetConfig(nil), model.Targets...)
		models[alias] = model
	}

	return gatewaySettings{
		maxBodyBytes:  cfg.Server.MaxBodyBytes,
		unknownModels: cfg.Catalog.UnknownModels,
		providerTypes: providerTypes,
		models:        models,
		routing:       cfg.Routing,
	}
}

func (s gatewaySettings) providerType(name string) string {
	return s.providerTypes[name]
}
