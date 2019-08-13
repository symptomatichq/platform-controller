package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/symptomatichq/kit/env"
	"github.com/symptomatichq/platform-controller/controller"
	clientset "github.com/symptomatichq/platform-controller/generated/clientset/versioned"
	informers "github.com/symptomatichq/platform-controller/generated/informers/externalversions"
	"github.com/symptomatichq/platform-controller/signals"
)

var (
	debug      *bool
	masterURL  *string
	kubeconfig *string
)

func main() {
	debug = flag.Bool("debug", env.Bool("DEBUG", false), "run the server in debug mode")
	kubeconfig = flag.String("kubeconfig", env.String("KUBE_CONFIG", ""), "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL = flag.String("master", env.String("MASTER_URL", ""), "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		klog.Fatalf("error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building kubernetes clientset: %s", err.Error())
	}

	symptomaticClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building symptomatic clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	symptomaticInformerFactory := informers.NewSharedInformerFactory(symptomaticClient, time.Second*30)

	controller := controller.NewController(kubeClient, symptomaticClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Extensions().V1beta1().Ingresses(),
		symptomaticInformerFactory.Symptomatic().V1alpha1().Applications(),
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	symptomaticInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("error running controller: %s", err.Error())
	}
}
