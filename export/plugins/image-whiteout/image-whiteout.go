package main

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/konveyor/crane-lib/transform"
	"github.com/konveyor/crane-lib/transform/cli"
)

func main() {
	cli.RunAndExit(cli.NewCustomPlugin("ImageWhiteoutPlugin", "v1", nil, Run))
}

func Run(request transform.PluginRequest) (transform.PluginResponse, error) {
	// plugin writers need to write custom code here.
	u := &request.Unstructured
	var patch jsonpatch.Patch
	var whiteout bool
	if u.GetKind() == "ImageStreamTag" {
		whiteout = true
	}
	if u.GetKind() == "ImageTag" {
		whiteout = true
	}
	return transform.PluginResponse{
		Version:    "v1",
		IsWhiteOut: whiteout,
		Patches:    patch,
	}, nil
}
