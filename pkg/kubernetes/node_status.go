package kubernetes

import (
	"fmt"
	goruntime "runtime"
	"time"

	"github.com/matishsiao/goInfo"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

const (
	// nodeStatusUpdateRetry specifies how many times kubelet retries when posting node status failed.
	nodeStatusUpdateRetry = 5

	LabelHostname           = "kubernetes.io/hostname"
	LabelZoneFailureDomain  = "failure-domain.beta.kubernetes.io/zone"
	LabelMultiZoneDelimiter = "__"
	LabelZoneRegion         = "failure-domain.beta.kubernetes.io/region"

	LabelInstanceType = "beta.kubernetes.io/instance-type"

	LabelOS   = "beta.kubernetes.io/os"
	LabelArch = "beta.kubernetes.io/arch"
)

// registerWithAPIServer registers the node with the cluster master. It is safe
// to call multiple times, but not concurrently (k.registrationCompleted is
// not locked).
func (k *Kubernetes) registerWithAPIServer() {
	if k.registrationCompleted {
		return
	}
	step := 100 * time.Millisecond

	for {
		time.Sleep(step)
		step = step * 2
		if step >= 7*time.Second {
			step = 7 * time.Second
		}

		node, err := k.initialNode()
		if err != nil {
			k.logger.Error().Msgf("Unable to construct v1.Node object for kubelet: %v", err)
			continue
		}

		k.logger.Info().Msgf("Attempting to register node %s", node.Name)
		registered := k.tryRegisterWithAPIServer(node)
		if registered {
			k.logger.Info().Msgf("Successfully registered node %s", node.Name)
			k.registrationCompleted = true
			return
		}
	}
}
func (k *Kubernetes) fakeRemoteControlPodName() string {
	return fmt.Sprintf("remote-control-%s", k.nodeName)
}

func (k *Kubernetes) fakeRemoteControlPod() *v1.Pod {
	now := metav1.NewTime(time.Now())
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              k.fakeRemoteControlPodName(),
			CreationTimestamp: now,
			Annotations: map[string]string{
				v1.MirrorPodAnnotationKey: "fake",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "container",
				Image: "fake/image:1.0",
			}},
			NodeName: k.nodeName,
			Tolerations: []v1.Toleration{{
				Effect:   v1.TaintEffectNoSchedule,
				Operator: v1.TolerationOpExists,
			}},
		},
	}
}

func (k *Kubernetes) setfakePodStatus(p *v1.Pod) {
	now := metav1.NewTime(time.Now())
	p.Name = k.fakeRemoteControlPodName()
	p.Status.StartTime = &now
	p.Status.Conditions = []v1.PodCondition{
		{
			LastTransitionTime: now,
			Status:             v1.ConditionTrue,
			Type:               v1.PodConditionType("PodScheduled"),
		},
		{
			LastTransitionTime: now,
			Status:             v1.ConditionTrue,
			Type:               v1.PodConditionType("Initialized"),
		},
		{
			Type:               v1.PodConditionType("Ready"),
			Status:             v1.ConditionTrue,
			LastTransitionTime: now,
		},
	}
	p.Status.ContainerStatuses = []v1.ContainerStatus{{
		Name:         "container",
		Ready:        true,
		ImageID:      "docker://deadcafe",
		ContainerID:  "docker://deadcafe",
		RestartCount: 0,
		State: v1.ContainerState{
			Running: &v1.ContainerStateRunning{
				StartedAt: now,
			},
		},
	}}
	p.Status.Phase = "Running"
	p.Status.HostIP = "1.1.1.1"
	p.Status.PodIP = "1.1.1.1"
}

// this ensure the fake pods are there
func (k *Kubernetes) ensureFakePods() error {
	cl := k.kubeClient.CoreV1().Pods(metav1.NamespaceDefault)
	name := k.fakeRemoteControlPodName()

	// remove any existing pod at first run
	if !k.fakePodsCompleted {
		var zero int64
		cl.Delete(name, &metav1.DeleteOptions{GracePeriodSeconds: &zero})
	}

	// try to get existing or get pod
	pod, err := cl.Get(name, metav1.GetOptions{})
	if err != nil {
		// other err than not existing
		if !apierrors.IsNotFound(err) {
			return err
		}
		pod, err = cl.Create(k.fakeRemoteControlPod())
		if err != nil {
			return err
		}
	}
	k.fakePodsCompleted = true

	k.setfakePodStatus(pod)
	_, err = cl.UpdateStatus(pod)
	return err

}

// tryRegisterWithAPIServer makes an attempt to register the given node with
// the API server, returning a boolean indicating whether the attempt was
// successful.  If a node with the same name already exists, it reconciles the
// value of the annotation for controller-managed attach-detach of attachable
// persistent volumes for the node.  If a node of the same name exists but has
// a different externalID value, it attempts to delete that node so that a
// later attempt can recreate it.
func (k *Kubernetes) tryRegisterWithAPIServer(node *v1.Node) bool {
	_, err := k.kubeClient.CoreV1().Nodes().Create(node)
	if err == nil {
		return true
	}

	if !apierrors.IsAlreadyExists(err) {
		k.logger.Error().Msgf("Unable to register node %q with API server: %v", k.nodeName, err)
		return false
	}

	existingNode, err := k.kubeClient.CoreV1().Nodes().Get(string(k.nodeName), metav1.GetOptions{})
	if err != nil {
		k.logger.Error().Msgf("Unable to register node %q with API server: error getting existing node: %v", k.nodeName, err)
		return false
	}
	if existingNode == nil {
		k.logger.Error().Msgf("Unable to register node %q with API server: no node instance returned", k.nodeName)
		return false
	}

	originalNode := existingNode.DeepCopy()
	if originalNode == nil {
		k.logger.Error().Msgf("Nil %q node object", k.nodeName)
		return false
	}

	if existingNode.Spec.ExternalID == node.Spec.ExternalID {
		k.Logger().Info().Msgf("Node %s was previously registered", k.nodeName)

		return true
	}

	k.Logger().Error().Msgf("Previously node %q had externalID %q; now it is %q; will delete and recreate.",
		k.nodeName, node.Spec.ExternalID, existingNode.Spec.ExternalID,
	)
	if err := k.kubeClient.CoreV1().Nodes().Delete(node.Name, nil); err != nil {
		k.Logger().Error().Msgf("Unable to register node %q with API server: error deleting old node: %v", k.nodeName, err)
	} else {
		k.Logger().Info().Msgf("Deleted old node object %q", k.nodeName)
	}

	return false
}

func (k *Kubernetes) initialNode() (*v1.Node, error) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(k.nodeName),
			Labels: map[string]string{
				LabelHostname:                    k.hostName,
				LabelOS:                          goruntime.GOOS,
				LabelArch:                        goruntime.GOARCH,
				"node-role.kubernetes.io/vacuum": "",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				v1.Taint{
					Key:    "mi.com/vacuum",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	}

	node.Spec.ExternalID = k.hostName

	k.setNodeStatus(node)

	return node, nil
}

func (k *Kubernetes) syncNodeStatus() {
	if k.kubeClient == nil {
		return
	}
	k.registerWithAPIServer()
	if err := k.ensureFakePods(); err != nil {
		k.logger.Error().Msgf("Unable to update fake pod status: %v", err)
	}
	if err := k.updateNodeStatus(); err != nil {
		k.logger.Error().Msgf("Unable to update node status: %v", err)
	}
}

// updateNodeStatus updates node status to master with retries.
func (k *Kubernetes) updateNodeStatus() error {
	for i := 0; i < nodeStatusUpdateRetry; i++ {
		if err := k.tryUpdateNodeStatus(i); err != nil {
			k.logger.Error().Msgf("Error updating node status, will retry: %v", err)
		} else {
			return nil
		}
	}
	return fmt.Errorf("update node status exceeds retry count")
}

// tryUpdateNodeStatus tries to update node status to master. If ReconcileCBR0
// is set, this function will also confirm that cbr0 is configured correctly.
func (k *Kubernetes) tryUpdateNodeStatus(tryNumber int) error {
	opts := metav1.GetOptions{}
	node, err := k.kubeClient.CoreV1().Nodes().Get(string(k.nodeName), opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", k.nodeName, err)
	}

	k.setNodeStatus(node)
	_, err = k.kubeClient.CoreV1().Nodes().UpdateStatus(node)
	if err != nil {
		return err
	}

	return nil
}

// Set Ready condition for the node.
func (k *Kubernetes) setNodeReadyCondition(node *v1.Node) {
	// NOTE(aaronlevy): NodeReady condition needs to be the last in the list of node conditions.
	// This is due to an issue with version skewed kubelet and master components.
	// ref: https://github.com/kubernetes/kubernetes/issues/16961
	currentTime := metav1.NewTime(time.Now())
	newNodeReadyCondition := v1.NodeCondition{
		Type:              v1.NodeReady,
		Status:            v1.ConditionTrue,
		Reason:            "KubeletReady",
		Message:           "kubelet is posting ready status",
		LastHeartbeatTime: currentTime,
	}

	readyConditionUpdated := false
	needToRecordEvent := false
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == v1.NodeReady {
			if node.Status.Conditions[i].Status == newNodeReadyCondition.Status {
				newNodeReadyCondition.LastTransitionTime = node.Status.Conditions[i].LastTransitionTime
			} else {
				newNodeReadyCondition.LastTransitionTime = currentTime
				needToRecordEvent = true
			}
			node.Status.Conditions[i] = newNodeReadyCondition
			readyConditionUpdated = true
			break
		}
	}
	if !readyConditionUpdated {
		newNodeReadyCondition.LastTransitionTime = currentTime
		node.Status.Conditions = append(node.Status.Conditions, newNodeReadyCondition)
	}
	if needToRecordEvent {
		if newNodeReadyCondition.Status == v1.ConditionTrue {
			k.recordNodeStatusEvent(v1.EventTypeNormal, "NodeReady")
		} else {
			k.recordNodeStatusEvent(v1.EventTypeNormal, "NodeUnready")
			k.logger.Info().Msgf("Node became not ready: %+v", newNodeReadyCondition)
		}
	}
}

func (k *Kubernetes) setNodeStatus(node *v1.Node) {
	k.setNodeStatusVersionInfo(node)
	k.setNodeAdress(node)
	k.setNodeReadyCondition(node)
	k.setNodeKubeletPort(node)
}

func (k *Kubernetes) setNodeKubeletPort(node *v1.Node) {
	node.Status.DaemonEndpoints.KubeletEndpoint.Port = int32(k.flags.Kubernetes.KubeletPort)
}

func (k *Kubernetes) setNodeAdress(node *v1.Node) {
	addrs := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: k.hostName},
	}

	ipAddr, err := utilnet.ChooseHostInterface()
	if err != nil {
		k.Logger().Warn().Msgf("Cloud not detect node's IP address: %s", err)
	} else {
		addrs = append(addrs, v1.NodeAddress{Type: v1.NodeInternalIP, Address: ipAddr.String()})
	}

	node.Status.Addresses = addrs
}

func (k *Kubernetes) setNodeStatusVersionInfo(node *v1.Node) {
	node.Status.NodeInfo.ContainerRuntimeVersion = fmt.Sprintf("%s://%s", "rockrobo", "v1")
	node.Status.NodeInfo.KubeletVersion = k.flags.Version.String()
	info := goInfo.GetInfo()
	node.Status.NodeInfo.KernelVersion = info.Kernel
	node.Status.NodeInfo.OperatingSystem = info.OS
}

// recordNodeStatusEvent records an event of the given type with the given
// message for the node.
func (k *Kubernetes) recordNodeStatusEvent(eventType, event string) {
	// TODO: log me to zerolog
	k.recorder.Eventf(k.nodeRef, eventType, event, "Node %s status is now: %s", k.nodeName, event)
}
