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
	"strings"
	"time"

	fluxsource "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SynManagedNameLabel      string = "managed.syn.servicenow.com/name"
	SynManagedNamespaceLabel string = "managed.syn.servicenow.com/namespace"
)

// SupervisedGitSpec defines the desired state of SupervisedGit
type SupervisedGitSpec struct {
	// TargetRef is the reference to an existing source to add
	// deploy controls to following a build, release, and deploy workflow.
	// +required
	AdoptTargetRef CrossNamespaceObjectReference `json:"targetRef,omitempty"`

	// Interval is the interval between reconciliation attempts.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default="30s"
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// ReconcileTimeLimit is the maximum time allowed for a reconciliation
	// before failing a change request
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +kubebuilder:default="5m"
	// +optional
	ReconcileTimeLimit *metav1.Duration `json:"reconcileTimeLimit,omitempty"`
}

type CrossNamespaceObjectReference struct {
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent. Currently only `GitRepository` is supported.
	// +kubebuilder:validation:Enum=GitRepository
	// +required
	Kind string `json:"kind"`

	// Name of the referent
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=53
	// +required
	Name string `json:"name"`

	// Namespace of the referent
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=53
	// +kubebuilder:validation:Optional
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Child Label Selector to match the child resources
	// created by or connected to the referent
	// +kubeuilder:validation:Optional
	// +optional
	ChildLabelSelector *metav1.LabelSelector `json:"childLabelSelector,omitempty"`
}

// SupervisedGitStatus defines the observed state of SupervisedGit
type SupervisedGitStatus struct {
	// ObservedGeneration is the most recent generation observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Last seen revision on the source
	// +optional
	LastSeenRevision string `json:"lastSeenRevision"`

	// Last fully applied revision
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision"`

	// Last approved revision
	// +optional
	LastApprovedRevision string `json:"lastApprovedRevision"`

	// Time starting to attempt last approved revision
	// +optional
	LastApprovedRevisionStart metav1.Time `json:"lastApprovedRevisionStart,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=sgit;sgits

// SupervisedGit is the Schema for the supervisedgits API
type SupervisedGit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SupervisedGitSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status SupervisedGitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SupervisedGitList contains a list of SupervisedGit
type SupervisedGitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SupervisedGit `json:"items"`
}

// GetLastSeenRevision returns the last seen revision on the source
func (s *SupervisedGit) GetLastSeenRevision() string {
	return s.Status.LastSeenRevision
}

// SetLastSeenRevision sets the last seen revision on the source
func (s *SupervisedGit) SetLastSeenRevision(revision string) {
	s.Status.LastSeenRevision = revision
}

// GetLastAppliedRevision returns the last fully applied revision
func (s *SupervisedGit) GetLastAppliedRevision() string {
	return s.Status.LastAppliedRevision
}

// SetLastAppliedRevision sets the last fully applied revision
func (s *SupervisedGit) SetLastAppliedRevision(revision string) {
	s.Status.LastAppliedRevision = revision
}

// GetLastApprovedRevision returns the last approved revision
func (s *SupervisedGit) GetLastApprovedRevision() string {
	return s.Status.LastApprovedRevision
}

// SetLastApprovedRevision sets the last approved revision
func (s *SupervisedGit) SetLastApprovedRevision(revision string) {
	s.Status.LastApprovedRevision = revision
}

// GetInterval returns the interval between reconciliation attempts
// if the interval is not set, it returns 30s
func (s *SupervisedGit) GetInterval() time.Duration {
	if s.Spec.Interval == nil {
		return 30 * time.Second
	}
	return s.Spec.Interval.Duration
}

// GetReconcileTimeLimit returns the maximum time allowed for a reconciliation
// before failing a change request
// if the time limit is not set, it returns 5m
func (s *SupervisedGit) GetReconcileTimeLimit() time.Duration {
	if s.Spec.ReconcileTimeLimit == nil {
		return 5 * time.Minute
	}
	return s.Spec.ReconcileTimeLimit.Duration
}

// GetLastApprovedRevisionStart returns the time starting to attempt last approved revision
// if the time is not set, it returns metav1.Now()
func (s *SupervisedGit) GetLastApprovedRevisionStart() metav1.Time {
	if s.Status.LastApprovedRevisionStart.IsZero() {
		return metav1.Now()
	}
	return s.Status.LastApprovedRevisionStart
}

// SetLastApprovedRevisionStart sets the time starting to attempt last approved revision
func (s *SupervisedGit) SetLastApprovedRevisionStart(time metav1.Time) {
	s.Status.LastApprovedRevisionStart = time
}

// SetLastApprovedRevisionStartNow sets the time starting to attempt last approved revision to now
func (s *SupervisedGit) SetLastApprovedRevisionStartNow() {
	s.Status.LastApprovedRevisionStart = metav1.Now()
}

func (s *SupervisedGit) GetLastGitrepoRevision(gitrepo fluxsource.GitRepository) string {
	if gitrepo.Status.Artifact != nil {
		if gitrepo.Status.Artifact.Revision != "" {
			return gitrepo.Status.Artifact.Revision
		}
	}
	return ""
}

func (s *SupervisedGit) GetLastGitrepoRevisionSHA(gitrepo fluxsource.GitRepository) string {
	str := s.GetLastGitrepoRevision(gitrepo)
	rev := strings.Split(str, ":")
	return rev[len(rev)-1]
}

func (s *SupervisedGit) GetRevisionSHA(revision string) string {
	rev := strings.Split(revision, ":")
	return rev[len(rev)-1]
}

func (s *SupervisedGit) GetAdopteesChildLabelSelectors() (labels map[string]string) {
	if s.Spec.AdoptTargetRef.ChildLabelSelector != nil {
		labels = s.Spec.AdoptTargetRef.ChildLabelSelector.MatchLabels
	}
	return labels
}

func (s SupervisedGit) GetConditions() []metav1.Condition {
	return s.Status.Conditions
}

func (s *SupervisedGit) SetConditions(conditions []metav1.Condition) {
	s.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&SupervisedGit{}, &SupervisedGitList{})
}
