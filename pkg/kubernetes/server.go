package kubernetes

import (
	"fmt"
	"net/http"
)

func (k *Kubernetes) helloServer(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
	k.logger.Debug().Interface("headers", req.Header).Str("path", req.RequestURI).Msg("server request unhandled")
	// fmt.Fprintf(w, "This is an example server.\n")
	// io.WriteString(w, "This is an example server.\n")
}

func (k *Kubernetes) runServer() {
	http.HandleFunc("/", k.helloServer)
	err := http.ListenAndServeTLS(fmt.Sprintf(":%d", k.flags.Kubernetes.KubeletPort), k.flags.Kubernetes.CertPath, k.flags.Kubernetes.KeyPath, nil)
	if err != nil {
		k.logger.Fatal().Err(err).Msg("ListenAndServe")
	}
}
