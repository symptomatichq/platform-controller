package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Application is a specification for a Application resource
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec"`
	Status ApplicationStatus `json:"status"`
}

// ApplicationSpec is the spec for a Application resource
type ApplicationSpec struct {
	Replicas     int32                        `json:"replicas"`
	Ingress      *ApplicationIngress          `json:"ingress"`
	LoadBalancer *ApplicationLoadBalancer     `json:"loadbalancer"`
	Containers   []ApplicationContainer       `json:"containers"`
	Secrets      map[string]ApplicationSecret `json:"secrets"`
	Config       map[string]string            `json:"config"`
}

// ApplicationIngress ...
type ApplicationIngress struct {
	Hostname string `json:"hostname"`
}

// ApplicationLoadBalancer ...
type ApplicationLoadBalancer struct {
	Port int32 `json:"port"`
}

// ApplicationContainer ...
type ApplicationContainer struct {
	Name  string                     `json:"name"`
	Image string                     `json:"image"`
	Ports []ApplicationContainerPort `json:"ports"`
}

// ApplicationContainerPort ...
type ApplicationContainerPort struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// ApplicationSecretType ...
type ApplicationSecretType string

const (
	// EnvType ...
	EnvType ApplicationSecretType = "env"
	// FileType ...
	FileType ApplicationSecretType = "file"
)

// ApplicationSecret ...
type ApplicationSecret struct {
	Key   string                `string:"key"`
	Value string                `string:"value"`
	Type  ApplicationSecretType `string:"type"`
}

// ApplicationStatus ...
type ApplicationStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApplicationList is a list of Application resources
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Application `json:"items"`
}
