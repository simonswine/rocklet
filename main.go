package main

import (
	"github.com/simonswine/sucklet/pkg/rockrobo"
)

func main() {
	r := rockrobo.New()
	err := r.Run()
	if err != nil {
		r.Logger().Fatal().Err(err).Msg("failed to run")
	}
}
