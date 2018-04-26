package kubernetes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/kr/pty"
	"k8s.io/apimachinery/pkg/types"
	remotecommandconsts "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/client-go/tools/remotecommand"
	remotecommandserver "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

func (k *Kubernetes) ExecInContainer(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to find my own executable: %s", err)
	}

	// Create arbitrary command.
	c := exec.Command(executable, "ui")

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle pty size.
	go func() {
		for s := range resize {
			err := pty.Setsize(ptmx, &pty.Winsize{
				Cols: s.Width,
				Rows: s.Height,
			})
			if err != nil {
				k.logger.Warn().Msgf("error resizing pty: %s", err)
			}
		}
	}()

	// Copy stdin to the pty and the pty to stdout.
	go func() { _, _ = io.Copy(ptmx, stdin) }()
	_, _ = io.Copy(stdout, ptmx)

	stderr.Close()
	stdout.Close()

	return nil
}

func (k *Kubernetes) remoteControlHandle(w http.ResponseWriter, req *http.Request) {
	streamOpts, err := remotecommandserver.NewOptions(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	remotecommandserver.ServeExec(
		w,
		req,
		k,
		k.fakeRemoteControlPodName(),
		types.UID("x"),
		"container",
		[]string{"cmd"},
		streamOpts,
		time.Minute,
		remotecommandconsts.DefaultStreamCreationTimeout,
		remotecommandconsts.SupportedStreamingProtocols,
	)
}

func (k *Kubernetes) runServer() {
	http.HandleFunc(fmt.Sprintf("/exec/default/remote-control-%s/container", k.nodeName), k.remoteControlHandle)
	err := http.ListenAndServeTLS(fmt.Sprintf(":%d", k.flags.Kubernetes.KubeletPort), k.flags.Kubernetes.CertPath, k.flags.Kubernetes.KeyPath, nil)
	if err != nil {
		k.logger.Fatal().Err(err).Msg("ListenAndServe")
	}
}
