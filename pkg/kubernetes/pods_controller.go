package kubernetes

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	api "k8s.io/kubernetes/pkg/apis/core"
)

type Controller struct {
	indexer    cache.Indexer
	queue      workqueue.RateLimitingInterface
	informer   cache.Controller
	logger     zerolog.Logger
	resource   string
	rockrobo   Rockrobo
	kubeClient kubernetes.Interface
}

func (k *Kubernetes) runPodController() {
	podListWatcher := cache.NewListWatchFromClient(
		k.kubeClient.CoreV1().RESTClient(),
		"pods",
		metav1.NamespaceAll,
		fields.OneTermEqualSelector(api.PodHostField, k.nodeName),
	)
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			// IndexerInformer uses a delta queue, therefore for deletes we have to use this
			// key function.
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})
	controller := k.NewController(queue, indexer, informer, "pods", k.kubeClient, k.rockrobo)

	controller.Run(1, k.stopCh)

}

func (k *Kubernetes) NewController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, resource string, kubeClient kubernetes.Interface, rockrobo Rockrobo) *Controller {
	return &Controller{
		informer: informer,
		indexer:  indexer,
		queue:    queue,
		logger:   k.logger,
		resource: resource,
	}
}

func (c *Controller) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer c.queue.Done(key)

	// Invoke the method containing the business logic
	err := c.sendNotify(key.(string))
	// Handle the error if something went wrong during the execution of the business logic
	c.handleErr(err, key)
	return true
}

func (c *Controller) sendNotify(key string) error {
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		c.logger.Warn().Msgf("fetching object with key %s from store failed with %v", key, err)
		return err
	}
	// pod got deleted
	if !exists {
		return nil
	}

	pod := obj.(*v1.Pod)

	// skip mirrored pods
	if _, ok := pod.Annotations[v1.MirrorPodAnnotationKey]; ok {
		return nil
	}
	if pod.Status.Phase == v1.PodPending {
		stopCh := make(chan struct{})
		done := make(chan bool)
		go func() {
			for pos, _ := range pod.Spec.Containers {
				var args json.RawMessage
				if len(pod.Spec.Containers[pos].Args) < 1 {
					args = json.RawMessage([]byte{})
				} else {
					args = json.RawMessage(pod.Spec.Containers[pos].Args[0])
				}
				err := c.rockrobo.AppCommand(
					pod.Spec.Containers[pos].Image,
					args,
				)
				if err != nil {
					done <- false
					return
				}
			}
			done <- true
		}()

		select {
		case <-time.Tick(5 * time.Second):
			close(stopCh)
			return fmt.Errorf("action on pod %s timed out", key)
		case result, ok := <-done:
			if ok && result {
				// TODO retry update status
				if err := c.updatePodStatus(pod, v1.PodSucceeded); err != nil {
					c.logger.Warn().Err(err).Msg("failed to update pod status")
				}
				return nil
			} else {
				if err := c.updatePodStatus(pod, v1.PodFailed); err != nil {
					c.logger.Warn().Err(err).Msg("failed to update pod status")
				}

				return fmt.Errorf("failed to run command pod: %s/%s", pod.Namespace, pod.Name)
			}
		}
	}

	c.logger.Info().Msgf("received update for pod %s/%s\n", pod.GetNamespace(), pod.GetName())
	//return fmt.Errorf("implement me: %s", "sendNotify")
	return nil
}

func (c *Controller) updatePodStatus(pod *v1.Pod, s v1.PodPhase) error {
	copy := pod.DeepCopy()
	copy.Status.Phase = s
	_, err := c.kubeClient.CoreV1().Pods(pod.Namespace).UpdateStatus(copy)
	return err
}

// handleErr checks if an error happened and makes sure we will retry later.
func (c *Controller) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		c.queue.Forget(key)
		return
	}

	// This controller retries 5 times if something goes wrong. After that, it stops trying.
	if c.queue.NumRequeues(key) < 5 {
		c.logger.Info().Msgf("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	runtime.HandleError(err)
	c.logger.Info().Msgf("Dropping %s %q out of the queue: %v", c.resource, key, err)
}

func (c *Controller) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	// Let the workers stop when we are done
	defer c.queue.ShutDown()
	c.logger.Info().Str("resource", c.resource).Msg("starting controller")

	go c.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	c.logger.Info().Str("resource", c.resource).Msg("stopping controller")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}
