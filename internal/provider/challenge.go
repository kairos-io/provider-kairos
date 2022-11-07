package provider

import (
	"encoding/json"

	"github.com/kairos-io/kairos/sdk/bus"

	"github.com/kairos-io/kairos/pkg/config"
	providerConfig "github.com/kairos-io/provider-kairos/internal/provider/config"

	"github.com/mudler/go-nodepair"
	"github.com/mudler/go-pluggable"
)

func Challenge(e *pluggable.Event) pluggable.EventResponse {
	p := &bus.EventPayload{}
	err := json.Unmarshal([]byte(e.Data), p)
	if err != nil {
		return ErrorEvent("Failed reading JSON input: %s input '%s'", err.Error(), e.Data)
	}

	cfg := &providerConfig.Config{}
	err = config.FromString(p.Config, cfg)
	if err != nil {
		return ErrorEvent("Failed reading JSON input: %s input '%s'", err.Error(), p.Config)
	}

	tk := ""
	if cfg.Kairos != nil && cfg.Kairos.NetworkToken != "" {
		tk = cfg.Kairos.NetworkToken
	}
	if tk == "" {
		tk = nodepair.GenerateToken()
	}
	return pluggable.EventResponse{
		Data: tk,
	}
}
