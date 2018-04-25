package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=vacuums

// Vacuum's store the outcome of a cleaning run
type Vacuum struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VacuumSpec   `json:"spec,omitempty"`
	Status VacuumStatus `json:"status,omitempty"`
}

const (
	VacuumStateUnknown       = "Unknown"
	VacuumStateInitiating    = "Initiating"
	VacuumStateSleeping      = "Sleeping"
	VacuumStateWaiting       = "Waiting"
	VacuumStateCleaning      = "Cleaning"
	VacuumStateReturningHome = "ReturningHome"
	VacuumStateCharging      = "Charging"
	VacuumStateChargingError = "ChargingError"
	VacuumStatePause         = "Pause"
	VacuumStateSpotCleaning  = "SpotCleaning"
	VacuumStatenError        = "Error"
	VacuumStateShuttingDown  = "ShuttingDown"
	VacuumStateUpdating      = "Updating"
	VacuumStateDocking       = "Docking"
	VacuumStateZoneCleaning  = "ZoneCleaning"
	VacuumStateFull          = "Full"
)

var VacuumStateHash = map[int]string{
	0:   VacuumStateUnknown,
	1:   VacuumStateInitiating,
	2:   VacuumStateSleeping,
	3:   VacuumStateWaiting,
	5:   VacuumStateCleaning,
	6:   VacuumStateReturningHome,
	8:   VacuumStateCharging,
	9:   VacuumStateChargingError,
	10:  VacuumStatePause,
	11:  VacuumStateSpotCleaning,
	12:  VacuumStatenError,
	13:  VacuumStateShuttingDown,
	14:  VacuumStateUpdating,
	15:  VacuumStateDocking,
	17:  VacuumStateZoneCleaning,
	100: VacuumStateFull,
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Vacuum is a list of Vacuums
type VacuumList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Vacuum `json:"items"`
}

type VacuumSpec struct {
	// position to drive to
	Position *Position `json:"path,omitempty"`

	// enable disable cloud
	Cloud bool `json:"cloud,omitempty"`
}

type VacuumStatus struct {
	State    string   `json:"state,omitempty"`
	Map      []byte   `json:map,omitempty""`
	Position Position `json:"path,omitempty"`
	Duration string   `json:"duration,omitempty"`
	Area     *int     `json:"area,omitempty"`
}

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
	Map          []byte       `json:"map,omitempty"`
	Path         []Position   `json:"path,omitempty"`
	Charger      Position     `json:"charger,omitempty"`
	BeginTime    *metav1.Time `json:"beginTime,omitempty"`
	EndTime      *metav1.Time `json:"endTime,omitempty"`
	DayBeginTime *metav1.Time `json:"dayBeginTime,omitempty"`
	Duration     string       `json:"duration,omitempty"`
	Area         *int         `json:"area,omitempty"`
	Complete     *bool        `json:"complete,omitempty"`
	Code         *int         `json:"code,omitempty"`
	ErrorCode    *int         `json:"errorCode,omitempty"`
}

type Position struct {
	X uint16 `json:"x,omitempty"`
	Y uint16 `json:"y,omitempty"`
}
