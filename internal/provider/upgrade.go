package provider

import (
	"encoding/json"

	"github.com/c3os-io/c3os/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mudler/go-pluggable"
)

func eventError(err error) pluggable.EventResponse {
	return pluggable.EventResponse{Error: err.Error()}
}

func ListVersions(e *pluggable.Event) pluggable.EventResponse {
	registry, err := utils.OSRelease("IMAGE_REPO")
	if err != nil {
		return eventError(err)
	}
	tags, err := crane.ListTags(registry)
	if err != nil {
		return eventError(err)
	}

	versions, err := json.Marshal(tags)
	resp := &pluggable.EventResponse{
		Data: string(versions),
	}

	if err != nil {
		resp.Error = err.Error()
	}

	return *resp
}
