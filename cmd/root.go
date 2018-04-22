package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

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
	u, err := user.Current()
	if err != nil {
		panic(err)
	}

	rootCmd.PersistentFlags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	// rockrobo flags
	rootCmd.PersistentFlags().StringVarP(&flags.DataDirectory, "data-directory", "d", "/mnt/data/miio", "MIIO data directory")
	rootCmd.PersistentFlags().StringVarP(&flags.RuntimeDirectory, "runtime-directory", "r", "/run/shm", "Runtime directory")
	rootCmd.PersistentFlags().StringVar(&flags.RobotDatabase, "robot-database", "/mnt/data/rockrobo/robot.db", "Path to robot's sqlite3 database")

	// kubernetes flags
	rootCmd.PersistentFlags().StringVar(&flags.Kubeconfig, "kubeconfig", filepath.Join(u.HomeDir, ".kube/config"), "Path to kubeconfig")
	rootCmd.PersistentFlags().IntVar(&flags.Kubernetes.KubeletPort, "kubelet-port", 10250, "Port for kubelet API to listen on")
	rootCmd.PersistentFlags().StringVar(&flags.Kubernetes.CertPath, "kube-cert-path", "rockrobo.pem", "Path to kube ceritificate")
	rootCmd.PersistentFlags().StringVar(&flags.Kubernetes.KeyPath, "kube-key-path", "rockrobo-key.pem", "Path to kube key")
	rootCmd.PersistentFlags().StringVar(&flags.Kubernetes.CACertPath, "kube-ca-cert-path", filepath.Join(u.HomeDir, ".minikube/ca.crt"), "Path to kube ca ceritificate")
	rootCmd.PersistentFlags().StringVar(&flags.Kubernetes.CAKeyPath, "kube-ca-key-path", filepath.Join(u.HomeDir, ".minikube/ca.key"), "Path to kube ca key")
	rootCmd.PersistentFlags().StringVar(&flags.Kubernetes.NodeName, "node-name", "rockrobo", "Node name to use for kubernetes")
}
