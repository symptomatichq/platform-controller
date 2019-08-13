// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	time "time"

	symptomaticv1alpha1 "github.com/symptomatichq/platform-controller/apis/symptomatic/v1alpha1"
	versioned "github.com/symptomatichq/platform-controller/generated/clientset/versioned"
	internalinterfaces "github.com/symptomatichq/platform-controller/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/symptomatichq/platform-controller/generated/listers/symptomatic/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ApplicationInformer provides access to a shared informer and lister for
// Applications.
type ApplicationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ApplicationLister
}

type applicationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewApplicationInformer constructs a new informer for Application type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewApplicationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredApplicationInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredApplicationInformer constructs a new informer for Application type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredApplicationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SymptomaticV1alpha1().Applications(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SymptomaticV1alpha1().Applications(namespace).Watch(options)
			},
		},
		&symptomaticv1alpha1.Application{},
		resyncPeriod,
		indexers,
	)
}

func (f *applicationInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredApplicationInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *applicationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&symptomaticv1alpha1.Application{}, f.defaultInformer)
}

func (f *applicationInformer) Lister() v1alpha1.ApplicationLister {
	return v1alpha1.NewApplicationLister(f.Informer().GetIndexer())
}
