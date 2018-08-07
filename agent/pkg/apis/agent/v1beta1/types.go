package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WeaveCloudList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []WeaveCloud `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type WeaveCloud struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              WeaveCloudSpec   `json:"spec"`
	Status            WeaveCloudStatus `json:"status,omitempty"`
}

type WeaveCloudSpec struct {
	General    GeneralSpec    `json:"general"`
	Flux       FluxSpec       `json:"flux"`
	Prometheus PrometheusSpec `json:"prometheus"`
	Scope      ScopeSpec      `json:"scope"`
}

type WeaveCloudStatus struct {
	// Fill me
}

type GeneralSpec struct {
	Autoupdate  bool   `json:"autoUpdate,omitempty""`
	Environment string `json:"environment,omitempty""`
}

type FluxSpec struct {
	Disable bool `json:"disable,omitempty"`
}

type PrometheusSpec struct {
	Disable         bool   `json:"disable,omitempty""`
	PodScrapePolicy string `json:"podScrapePolicy,omitempty"`
}

type ScopeSpec struct {
	Disable                  bool   `json:"disable,omitempty""`
	ReadOnly                 bool   `json:"readOnly,omitempty"`
	ContainerRuntimeEndpoint string `json:"containerRuntimeEndpoint,omitempty""`
}
