package api

import (
	"fmt"
)

type Version struct {
	AppVersion string
	CommitHash string
	GitState   string
}

func (v *Version) String() string {
	if v.GitState != "clean" {
		return fmt.Sprintf("v%s-%.*s-%s", v.AppVersion, 8, v.CommitHash, v.GitState)
	}
	return fmt.Sprintf("v%s-%.*s", v.AppVersion, 8, v.CommitHash)
}

type Flags struct {
	Verbose          bool
	DataDirectory    string
	RuntimeDirectory string
	RobotDatabase    string

	Cloud struct {
		Enabled bool
	}

	Kubernetes struct {
		Enabled     bool
		Kubeconfig  string
		KubeletPort int

		CertPath   string
		KeyPath    string
		CACertPath string
		CAKeyPath  string

		NodeName string
	}

	Version *Version
}
