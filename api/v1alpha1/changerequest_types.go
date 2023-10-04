/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ChangeRequestOpened is the status of a change request that has been opened in the external system
	ChangeRequestOpened string = "opened"

	// ChangeRequestApproved is the status of a change request that has been approved in the external system
	ChangeRequestApproved string = "approved"

	// ChangeRequestRejected is the status of a change request that has been rejected in the external system
	ChangeRequestRejected string = "rejected"

	// ChangeRequestClosed is the status of a change request that has been closed in the external system
	ChangeRequestClosed string = "closed"

	// CloseSuccess is used to indicate that a change was reconciled and the change request should be closed as successful
	CloseSuccess string = "success"

	// CloseFail is used to indicate that a change failed to reconcile or reconciled with errors and the change request should be closed as failed
	CloseFail string = "fail"
)

// ChangeRequestSpec defines the desired state of ChangeRequest
type ChangeRequestSpec struct {
	// ChgTemplateName is the name of the change request template
	// which is used for Normal or Standard change request
	// +optional
	ChgTemplateName string `json:"chgTemplateName,omitempty"`

	// ChgTemplateValues is a Key/value pairs to populate the CHG template
	// used for opening change requests
	// +optional
	ChgTemplateValues map[string]string `json:"chgTemplateValues,omitempty"`

	// CurrentRevision is the currently applied revision of the workload
	// If no revision currently applied should be set to None
	// +optional
	CurrentRevision string `json:"currentRevision,omitempty"`

	// TargetRevision is the revision to apply to the workload
	// +required
	TargetRevision string `json:"targetRevision"`

	// Close code is the code to close a change request with
	// +optional
	// +kubebuilder:validation:Enum:=success;fail
	CloseCode string `json:"closeCode,omitempty"`

	// Workload or project that the change is requested for
	// If not supplied will be inferred from the namespace
	// +optional
	Workload string `json:"workload,omitempty"`
}

// ChangeRequestStatus defines the observed state of ChangeRequest
type ChangeRequestStatus struct {
	// ObservedGeneration is the last observed generation of the changerequest
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// RequestPhase is the overall status for the changerequest
	// +optional
	RequestPhase string `json:"requestPhase,omitempty"`

	// ApprovedRevision is the revision that was approved by the change request approver.
	// This will be either the revision requested if approved or empty if the change request was not approved.
	// +optional
	ApprovedRevision string `json:"approvedRevision,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=chg;chgs
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.spec.targetRevision`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// ChangeRequest is the Schema for the changerequests API
type ChangeRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ChangeRequestSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status ChangeRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ChangeRequestList contains a list of ChangeRequest
type ChangeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChangeRequest `json:"items"`
}

func (c *ChangeRequest) GetRequestPhase() string {
	return c.Status.RequestPhase
}

func (c *ChangeRequest) SetRequestPhase(phase string) {
	c.Status.RequestPhase = phase
}

func (c *ChangeRequest) GetApprovedRevision() string {
	return c.Status.ApprovedRevision
}

func (c *ChangeRequest) SetApprovedRevision(revision string) {
	c.Status.ApprovedRevision = revision
}

func init() {
	SchemeBuilder.Register(&ChangeRequest{}, &ChangeRequestList{})
}
