package provider

import (
	"encoding/json"

	"github.com/kairos-io/kairos-sdk/bus"

	"github.com/kairos-io/kairos-agent/v2/pkg/config"
	providerConfig "github.com/kairos-io/provider-kairos/v2/internal/provider/config"

	"github.com/kairos-io/go-nodepair"
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
	if cfg.P2P != nil && cfg.P2P.NetworkToken != "" {
		tk = cfg.P2P.NetworkToken
	}
	if tk == "" {
		tk = nodepair.GenerateToken()
	}
	return pluggable.EventResponse{
		Data: tk,
	}
}
