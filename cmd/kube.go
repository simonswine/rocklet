package cmd

import (
	"github.com/spf13/cobra"

	"github.com/simonswine/rocklet/pkg/kubernetes"
)

var kubeCmd = &cobra.Command{
	Use:   "kube",
	Short: "Kubecontroller acting as fake kubelet",
	Run: func(cmd *cobra.Command, args []string) {
		k := kubernetes.New(flags)
		err := k.Run()
		if err != nil {
			k.Logger().Fatal().Err(err).Msg("failed to run")
		}
	},
}

func init() {
	rootCmd.AddCommand(kubeCmd)
}
