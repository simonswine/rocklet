package api

type Flags struct {
	Verbose          bool
	DataDirectory    string
	RuntimeDirectory string
	Kubeconfig       string
	RobotDatabase    string

	Kubernetes struct {
		KubeletPort int

		CertPath   string
		KeyPath    string
		CACertPath string
		CAKeyPath  string

		NodeName string
	}
}
