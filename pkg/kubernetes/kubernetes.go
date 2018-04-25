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
	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
	vacuum "github.com/simonswine/rocklet/pkg/client/clientset/versioned/typed/vacuum/v1alpha1"
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

	kubeClient   kubernetes.Interface
	vacuumClient vacuum.VacuumV1alpha1Interface

	// for internal book keeping; access only from within registerWithApiserver
	registrationCompleted bool

	// channel for vacuum status updates
	vacuumStatusCh chan v1alpha1.VacuumStatus

	// channel for cleanings
	cleaningCh chan *v1alpha1.Cleaning

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

	config, err := clientcmd.BuildConfigFromFlags("", k.flags.Kubernetes.Kubeconfig)
	if err != nil {
		return err
	}

	k.kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	k.vacuumClient, err = vacuum.NewForConfig(config)
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

	nodeStatusTicker := time.NewTicker(15 * time.Second)

	if k.cleaningCh == nil {
		k.cleaningCh = make(chan *v1alpha1.Cleaning)
	}

	for {
		select {
		case c := <-k.cleaningCh:
			go func() {
				err := k.ensureCleaning(c)
				if err != nil {
					k.Logger().Warn().Err(err).Msgf("failed to upload cleaning %s", c.Name)
				}
			}()
		case <-nodeStatusTicker.C:
			k.logger.Debug().Msg("sync node status")
			k.syncNodeStatus()
		case <-k.stopCh:
			break
		}
	}

	k.logger.Info().Msg("kubernetes is exiting")

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

func NewInternal(flags *api.Flags, stopCh chan struct{}, statusCh chan v1alpha1.VacuumStatus, cleaningCh chan *v1alpha1.Cleaning) *Kubernetes {
	k := New(flags)
	k.stopCh = stopCh
	k.vacuumStatusCh = statusCh
	k.cleaningCh = cleaningCh
	return k
}
