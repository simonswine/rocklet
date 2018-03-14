package cmd

import (
	"github.com/spf13/cobra"

	"github.com/simonswine/rocklet/pkg/navmap"
)

var navmapCmd = &cobra.Command{
	Use:   "navmap",
	Short: "Webserver exposing a dynamic map, while the vacuum is moving",
	Run: func(cmd *cobra.Command, args []string) {
		n := navmap.New(flags)
		err := n.Run()
		if err != nil {
			n.Logger().Fatal().Err(err).Msg("failed to run")
		}
	},
}

func init() {
	rootCmd.AddCommand(navmapCmd)
}
