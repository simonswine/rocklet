package api

type Flags struct {
	Verbose          bool
	DataDirectory    string
	RuntimeDirectory string
	RobotDatabase    string
	AppProxyMapPath  string

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
}
