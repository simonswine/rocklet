package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=cleanings

// Cleaning's store the outcome of a cleaning run
type Cleaning struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CleaningSpec   `json:"spec,omitempty"`
	Status CleaningStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cleaning is a list of Cleanings
type CleaningList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cleaning `json:"items"`
}

type CleaningSpec struct {
	NodeName string `json:"nodeName,omitempty"`
}

type CleaningStatus struct {
	State        string       `json:"state,omitempty"`
	Map          []byte       `json:map,omitempty""`
	Path         []Position   `json:"path,omitempty"`
	BeginTime    *metav1.Time `json:"beginTime,omitempty"`
	EndTime      *metav1.Time `json:"endTime,omitempty"`
	DayBeginTime *metav1.Time `json:"dayBeginTime,omitempty"`
}

type Position struct {
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
}
