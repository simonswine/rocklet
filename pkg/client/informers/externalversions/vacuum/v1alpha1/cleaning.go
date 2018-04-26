/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	vacuum_v1alpha1 "github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
	versioned "github.com/simonswine/rocklet/pkg/client/clientset/versioned"
	internalinterfaces "github.com/simonswine/rocklet/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/simonswine/rocklet/pkg/client/listers/vacuum/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CleaningInformer provides access to a shared informer and lister for
// Cleanings.
type CleaningInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CleaningLister
}

type cleaningInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCleaningInformer constructs a new informer for Cleaning type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCleaningInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCleaningInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCleaningInformer constructs a new informer for Cleaning type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCleaningInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.VacuumV1alpha1().Cleanings(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.VacuumV1alpha1().Cleanings(namespace).Watch(options)
			},
		},
		&vacuum_v1alpha1.Cleaning{},
		resyncPeriod,
		indexers,
	)
}

func (f *cleaningInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCleaningInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cleaningInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&vacuum_v1alpha1.Cleaning{}, f.defaultInformer)
}

func (f *cleaningInformer) Lister() v1alpha1.CleaningLister {
	return v1alpha1.NewCleaningLister(f.Informer().GetIndexer())
}