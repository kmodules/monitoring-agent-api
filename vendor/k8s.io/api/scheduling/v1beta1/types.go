/*
Copyright 2018 The Kubernetes Authors.

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

package v1beta1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.11
// +k8s:prerelease-lifecycle-gen:deprecated=1.14
// +k8s:prerelease-lifecycle-gen:removed=1.22
// +k8s:prerelease-lifecycle-gen:replacement=scheduling.k8s.io,v1,PriorityClass

// DEPRECATED - This group version of PriorityClass is deprecated by scheduling.k8s.io/v1/PriorityClass.
// PriorityClass defines mapping from a priority class name to the priority
// integer value. The value can be any valid integer.
type PriorityClass struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The value of this priority class. This is the actual priority that pods
	// receive when they have the name of this class in their pod spec.
	Value int32 `json:"value"`

	// globalDefault specifies whether this PriorityClass should be considered as
	// the default priority for pods that do not have any priority class.
	// Only one PriorityClass can be marked as `globalDefault`. However, if more than
	// one PriorityClasses exists with their `globalDefault` field set to true,
	// the smallest value of such global default PriorityClasses will be used as the default priority.
	// +optional
	GlobalDefault bool `json:"globalDefault,omitempty"`

	// description is an arbitrary string that usually provides guidelines on
	// when this priority class should be used.
	// +optional
	Description string `json:"description,omitempty"`

	// PreemptionPolicy is the Policy for preempting pods with lower priority.
	// One of Never, PreemptLowerPriority.
	// Defaults to PreemptLowerPriority if unset.
	// +optional
	PreemptionPolicy *apiv1.PreemptionPolicy `json:"preemptionPolicy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:prerelease-lifecycle-gen:introduced=1.11
// +k8s:prerelease-lifecycle-gen:deprecated=1.14
// +k8s:prerelease-lifecycle-gen:removed=1.22
// +k8s:prerelease-lifecycle-gen:replacement=scheduling.k8s.io,v1,PriorityClassList

// PriorityClassList is a collection of priority classes.
type PriorityClassList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// items is the list of PriorityClasses
	Items []PriorityClass `json:"items"`
}
