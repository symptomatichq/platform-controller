package controller

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	extensionsinformers "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	extensionslisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	symptomaticv1alpha1 "github.com/symptomatichq/platform-controller/apis/symptomatic/v1alpha1"
	clientset "github.com/symptomatichq/platform-controller/generated/clientset/versioned"
	symptomaticscheme "github.com/symptomatichq/platform-controller/generated/clientset/versioned/scheme"
	informers "github.com/symptomatichq/platform-controller/generated/informers/externalversions/symptomatic/v1alpha1"
	listers "github.com/symptomatichq/platform-controller/generated/listers/symptomatic/v1alpha1"
)

const controllerAgentName = "symptomatic-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Application is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Application fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Application"
	// MessageResourceSynced is the message used for an Event fired when a Application
	// is synced successfully
	MessageResourceSynced = "Application synced successfully"
)

// Controller is the controller implementation for Application resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// symptomaticclientset is a clientset for our own API group
	symptomaticclientset clientset.Interface

	ingressesLister    extensionslisters.IngressLister
	ingressesSynced    cache.InformerSynced
	servicesLister     corelisters.ServiceLister
	servicesSynced     cache.InformerSynced
	deploymentsLister  appslisters.DeploymentLister
	deploymentsSynced  cache.InformerSynced
	ApplicationsLister listers.ApplicationLister
	ApplicationsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new symptomatic controller
func NewController(
	kubeclientset kubernetes.Interface,
	symptomaticclientset clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	serviceInformer coreinformers.ServiceInformer,
	ingressInformer extensionsinformers.IngressInformer,
	ApplicationInformer informers.ApplicationInformer) *Controller {

	// Create event broadcaster
	// Add symptomatic-controller types to the default Kubernetes Scheme so Events can be
	// logged for symptomatic-controller types.
	utilruntime.Must(symptomaticscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:        kubeclientset,
		symptomaticclientset: symptomaticclientset,
		ingressesLister:      ingressInformer.Lister(),
		ingressesSynced:      ingressInformer.Informer().HasSynced,
		servicesLister:       serviceInformer.Lister(),
		servicesSynced:       serviceInformer.Informer().HasSynced,
		deploymentsLister:    deploymentInformer.Lister(),
		deploymentsSynced:    deploymentInformer.Informer().HasSynced,
		ApplicationsLister:   ApplicationInformer.Lister(),
		ApplicationsSynced:   ApplicationInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Applications"),
		recorder:             recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Application resources change
	ApplicationInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueApplication,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueApplication(new)
		},
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Application resource will enqueue that Application resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Application controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.servicesSynced, c.deploymentsSynced, c.ApplicationsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Application resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var (
			key string
			ok  bool
		)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call Forget here else
			// we'd go into a loop of attempting to process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Application resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Application resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Application resource with this namespace/name
	Application, err := c.ApplicationsLister.Applications(namespace).Get(name)
	if err != nil {
		// The Application resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("Application '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// Get the deployment with the name specified in Application.spec
	deployment, err := c.deploymentsLister.Deployments(Application.Namespace).Get(Application.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(Application.Namespace).Create(newDeployment(Application))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Application resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(deployment, Application) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(Application, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the Application resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if Application.Spec.Replicas != *deployment.Spec.Replicas {
		klog.V(4).Infof("Application %s replicas: %d, deployment replicas: %d", name, Application.Spec.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.kubeclientset.AppsV1().Deployments(Application.Namespace).Update(newDeployment(Application))
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Get the service with the name specified in Application.spec
	service, err := c.servicesLister.Services(Application.Namespace).Get(Application.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = c.kubeclientset.CoreV1().Services(Application.Namespace).Create(newService(Application))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Get the ingress with the name specified in Application.spec
	ingress, err := c.ingressesLister.Ingresses(Application.Namespace).Get(Application.Name)
	// If the resource doesn't exist, and we need to, we'll create it
	if errors.IsNotFound(err) && Application.Spec.Ingress != nil {
		ingress, err = c.kubeclientset.ExtensionsV1beta1().Ingresses(Application.Namespace).Create(newIngress(Application))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Application resource to reflect the
	// current state of the world
	err = c.updateApplicationStatus(Application, deployment, service, ingress)
	if err != nil {
		return err
	}

	c.recorder.Event(Application, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateApplicationStatus(Application *symptomaticv1alpha1.Application, deployment *appsv1.Deployment, service *corev1.Service, ing *extensionsv1beta1.Ingress) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	ApplicationCopy := Application.DeepCopy()
	ApplicationCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Application resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.symptomaticclientset.SymptomaticV1alpha1().Applications(Application.Namespace).Update(ApplicationCopy)
	return err
}

// enqueueApplication takes a Application resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Application.
func (c *Controller) enqueueApplication(obj interface{}) {
	var (
		key string
		err error
	)

	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.workqueue.Add(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Application resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Application resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var (
		object metav1.Object
		ok     bool
	)

	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Application, we should not do anything more
		// with it.
		if ownerRef.Kind != "Application" {
			return
		}

		Application, err := c.ApplicationsLister.Applications(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of Application '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueApplication(Application)
		return
	}
}

// newDeployment creates a new Deployment for a Application resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Application resource that 'owns' it.
func newDeployment(Application *symptomaticv1alpha1.Application) *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name":       Application.Name,
		"app.kubernetes.io/component":  "deployment",
		"app.kubernetes.io/managed-by": "platform-controller",
	}

	containers := []corev1.Container{}
	for _, ctr := range Application.Spec.Containers {
		ctrPorts := []corev1.ContainerPort{}
		for _, p := range ctr.Ports {
			ctrPorts = append(ctrPorts, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: p.Port,
			})
		}
		containers = append(containers, corev1.Container{
			Name:  ctr.Name,
			Image: ctr.Image,
			Ports: ctrPorts,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Application.Name,
			Namespace: Application.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(Application, symptomaticv1alpha1.SchemeGroupVersion.WithKind("Application")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &(Application.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

// newService creates a new Service for a Application resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Application resource that 'owns' it.
func newService(Application *symptomaticv1alpha1.Application) *corev1.Service {
	labels := map[string]string{
		"app.kubernetes.io/name":       Application.Name,
		"app.kubernetes.io/component":  "service",
		"app.kubernetes.io/managed-by": "platform-controller",
	}

	targetLabels := map[string]string{
		"app.kubernetes.io/name":       Application.Name,
		"app.kubernetes.io/component":  "deployment",
		"app.kubernetes.io/managed-by": "platform-controller",
	}

	svcPorts := []corev1.ServicePort{}
	for _, ctr := range Application.Spec.Containers {
		for _, p := range ctr.Ports {
			svcPorts = append(svcPorts, corev1.ServicePort{
				Name:       p.Name,
				Port:       p.Port,
				TargetPort: intstr.FromString(p.Name),
			})
		}
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Application.Name,
			Namespace: Application.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(Application, symptomaticv1alpha1.SchemeGroupVersion.WithKind("Application")),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports:    svcPorts,
			Selector: targetLabels,
		},
	}
}

// newIngress creates a new Ingress for a Application resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Application resource that 'owns' it.
func newIngress(Application *symptomaticv1alpha1.Application) *extensionsv1beta1.Ingress {
	labels := map[string]string{
		"app.kubernetes.io/name":       Application.Name,
		"app.kubernetes.io/component":  "ingress",
		"app.kubernetes.io/managed-by": "platform-controller",
	}

	return &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Application.Name,
			Namespace: Application.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(Application, symptomaticv1alpha1.SchemeGroupVersion.WithKind("Application")),
			},
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: Application.Spec.Ingress.Hostname,
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path: "/",
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: Application.Name,
										ServicePort: intstr.FromString("http"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
