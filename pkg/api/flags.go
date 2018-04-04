package api

type Flags struct {
	Verbose          bool
	DataDirectory    string
	RuntimeDirectory string
	Kubeconfig       string

	KubeAPIPort int
}
