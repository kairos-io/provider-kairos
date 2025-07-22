package provider

import (
	"encoding/json"
	"github.com/kairos-io/kairos-sdk/types"
	"github.com/mudler/go-pluggable"
)

// BuildEvent handles the buildtime event for the provider. Called by kairos-init during the build process.
func BuildEvent(e *pluggable.Event) pluggable.EventResponse {
	l := types.NewKairosLogger("provider-kairos-build", "info", true)
	l.Logger.Info().Msg("Buildtime event received")
	l.Logger.Debug().Interface("event", e).Msg("Event details")
	// unmarshal the event data if needed
	p := &ProviderPayload{}
	if e.Data != "" {
		err := json.Unmarshal([]byte(e.Data), p)
		if err != nil {
			l.Logger.Error().Err(err).Msg("Failed to unmarshal event data")
			return pluggable.EventResponse{
				State: "",
				Data:  "",
				Error: err.Error(),
			}
		}
	}
	// Now move the logger to the requested log level
	l.SetLevel(p.LogLevel)
	l.Logger.Debug().Interface("payload", p).Msg("Payload details")
	// Here you can access the provider, version, log level, and config from the payload
	l.Logger.Debug().Str("provider", p.Provider).Msg("Provider requested")
	l.Logger.Debug().Str("version", p.Version).Msg("Version requested")
	l.Logger.Debug().Str("logLevel", p.LogLevel).Msg("Log level requested")
	l.Logger.Debug().Str("config", p.Config).Msg("Config file requested")

	// Do the buildtime logic here for the given provider

	// This is the returned data for the buildtime event
	data := pluggable.EventResponse{
		State: "",
		Data:  "",
		Error: "",
	}

	l.Logger.Debug().Msg("Returning response for buildtime event")
	l.Logger.Debug().Interface("response", data).Msg("Response details")

	return data
}

type ProviderPayload struct {
	Provider string `json:"provider"` // What provider the user requested
	Version  string `json:"version"`  // What version of the provider, can be empty to signal latest
	LogLevel string `json:"logLevel"` // The log level to use for the provider
	Config   string `json:"config"`   // The config file to pass to the provider, can be empty if not needed
}
