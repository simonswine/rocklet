package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/simonswine/rocklet/pkg/api"
	"github.com/simonswine/rocklet/pkg/rockrobo"
)

var flags = &api.Flags{}

var rootCmd = &cobra.Command{
	Use:   "rocklet",
	Short: "Rocklet is a tool for rockrobo",
	Run: func(cmd *cobra.Command, args []string) {
		r := rockrobo.New(flags)
		err := r.Run()
		if err != nil {
			r.Logger().Fatal().Err(err).Msg("failed to run")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVarP(&flags.DataDirectory, "data-directory", "d", "/mnt/data/miio", "MIIO data directory")
	rootCmd.PersistentFlags().StringVarP(&flags.RuntimeDirectory, "runtime-directory", "r", "/run/shm", "Runtime directory")
}
