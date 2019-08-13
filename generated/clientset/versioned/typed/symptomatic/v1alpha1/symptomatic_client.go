// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/symptomatichq/platform-controller/apis/symptomatic/v1alpha1"
	"github.com/symptomatichq/platform-controller/generated/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type SymptomaticV1alpha1Interface interface {
	RESTClient() rest.Interface
	ApplicationsGetter
}

// SymptomaticV1alpha1Client is used to interact with features provided by the symptomatic.com group.
type SymptomaticV1alpha1Client struct {
	restClient rest.Interface
}

func (c *SymptomaticV1alpha1Client) Applications(namespace string) ApplicationInterface {
	return newApplications(c, namespace)
}

// NewForConfig creates a new SymptomaticV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*SymptomaticV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SymptomaticV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new SymptomaticV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SymptomaticV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SymptomaticV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *SymptomaticV1alpha1Client {
	return &SymptomaticV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *SymptomaticV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
