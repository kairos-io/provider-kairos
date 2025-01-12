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
			YAMLSection: "p2p.network_token",
			Prompt:      "Insert a network token, leave empty to autogenerate",
			AskFirst:    true,
			AskPrompt:   "Do you want to set up a full mesh-support?",
			IfEmpty:     node.GenerateNewConnectionData().Base64(),
		},
		{
			YAMLSection: "k3s.enabled",
			Bool:        true,
			Prompt:      "Do you want to enable k3s? (Select either k3s or k0s)",
		},
		{
			YAMLSection: "k0s.enabled",
			Bool:        true,
			Prompt:      "Do you want to enable k0s? (Select either k3s or k0s)",
		},
	}

	// Check for conflicts between k3s and k0s
	if k3sEnabled, k0sEnabled := checkK3sK0s(prompts); k3sEnabled && k0sEnabled {
		// Conflict detected, prompt user to resolve it
		prompts = append(prompts,
			bus.YAMLPrompt{
				YAMLSection: "k3s.enabled",
				Bool:        false,
				Prompt:      "Both k3s and k0s are enabled. Do you want to disable k3s?",
			},
			bus.YAMLPrompt{
				YAMLSection: "k0s.enabled",
				Bool:        false,
				Prompt:      "Both k3s and k0s are enabled. Do you want to disable k0s?",
			},
		)
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

func checkK3sK0s(prompts []bus.YAMLPrompt) (bool, bool) {
	var k3sEnabled, k0sEnabled bool

	for _, prompt := range prompts {
		switch prompt.YAMLSection {
		case "k3s.enabled":
			k3sEnabled = prompt.Bool
		case "k0s.enabled":
			k0sEnabled = prompt.Bool
		}
	}

	return k3sEnabled, k0sEnabled
}
