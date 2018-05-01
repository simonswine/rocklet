package main

import (
	"github.com/simonswine/rocklet/cmd"
	"github.com/simonswine/rocklet/pkg/api"
)

var appVersion string
var commitHash string
var gitState string

func main() {
	cmd.Execute(&api.Version{
		AppVersion: appVersion,
		CommitHash: commitHash,
		GitState:   gitState,
	})
}
