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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SupervisedGitSpec defines the desired state of SupervisedGit
type SupervisedGitSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of SupervisedGit. Edit supervisedgit_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// SupervisedGitStatus defines the observed state of SupervisedGit
type SupervisedGitStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SupervisedGit is the Schema for the supervisedgits API
type SupervisedGit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SupervisedGitSpec   `json:"spec,omitempty"`
	Status SupervisedGitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SupervisedGitList contains a list of SupervisedGit
type SupervisedGitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SupervisedGit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SupervisedGit{}, &SupervisedGitList{})
}
