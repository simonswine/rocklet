package kubernetes

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"

	"github.com/simonswine/rocklet/pkg/api"
)

type Kubernetes struct {
	stopCh   chan struct{}
	logger   zerolog.Logger
	flags    *api.Flags
	nodeName string
	hostName string

	// The EventRecorder to use
	recorder record.EventRecorder

	// Reference to this node.
	nodeRef *v1.ObjectReference

	kubeClient kubernetes.Interface

	// for internal book keeping; access only from within registerWithApiserver
	registrationCompleted bool

	fakePodsCompleted bool
}

func (k *Kubernetes) Logger() *zerolog.Logger {
	return &k.logger
}

func (k *Kubernetes) Run() error {
	var err error

	go k.runServer()

	if k.hostName == "" {
		k.hostName, err = os.Hostname()
		if err != nil {
			k.hostName = k.nodeName
		}
	}

	k.nodeRef = &v1.ObjectReference{
		Kind:      "Node",
		Name:      string(k.nodeName),
		UID:       types.UID(k.nodeName),
		Namespace: "",
	}

	config, err := clientcmd.BuildConfigFromFlags("", k.flags.Kubeconfig)
	if err != nil {
		return err
	}

	k.kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: k.kubeClient.CoreV1().Events(""),
	})
	k.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{
		Component: "rocklet",
		Host:      k.nodeName,
	})

	for {
		k.logger.Debug().Msg("sync node status")
		k.syncNodeStatus()
		time.Sleep(15 * time.Second)
	}

	return nil

}

func New(flags *api.Flags) *Kubernetes {
	k := &Kubernetes{
		nodeName: flags.Kubernetes.NodeName,
		logger: zerolog.New(os.Stdout).With().
			Str("app", "kubernetes").
			Logger().Level(zerolog.DebugLevel),
		flags: flags,
	}
	return k
}
