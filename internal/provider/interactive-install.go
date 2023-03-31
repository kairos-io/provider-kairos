package provider

import (
	"encoding/json"

	"github.com/kairos-io/kairos-sdk/bus"

	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/go-pluggable"
)

func InteractiveInstall(e *pluggable.Event) pluggable.EventResponse { //nolint:revive
	prompts := []bus.YAMLPrompt{
		{
			YAMLSection: "kairos.network_token",
			Prompt:      "Insert a network token, leave empty to autogenerate",
			AskFirst:    true,
			AskPrompt:   "Do you want to setup a full mesh-support?",
			IfEmpty:     node.GenerateNewConnectionData().Base64(),
		},
		{
			YAMLSection: "k3s.enabled",
			Bool:        true,
			Prompt:      "Do you want to enable k3s?",
		},
	}
	payload, err := json.Marshal(prompts)
	if err != nil {
		return ErrorEvent("Failed marshalling JSON input: %s", err.Error())
	}

	return pluggable.EventResponse{
		State: "",
		Data:  string(payload),
		Error: "",
	}
}
