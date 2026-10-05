/*
Copyright 2026.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExposeConfig defines public access settings for the application.
type ExposeConfig struct {
	// Port is the container port to expose.
	// +required
	Port int32 `json:"port"`

	// IngressClassName is the name of the IngressClass to use for the application's Ingress.
	// +required
	IngressClassName string `json:"ingressClassName"`

	// Domain is the hostname to expose (e.g. app.example.com).
	// +required
	Domain string `json:"domain"`
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// Image is the container image to deploy (e.g. ghcr.io/org/app:tag).
	// +required
	Image string `json:"image"`

	// Build is the user-selected image build configuration.
	// +optional
	Build *BuildConfig `json:"build,omitempty"`

	// Env is a list of environment variables for the runtime container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Expose defines public access settings.
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`
}

// ApplicationStatus defines the observed state of Application.
type ApplicationStatus struct {
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=app

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ApplicationSpec `json:"spec"`

	// +optional
	Status ApplicationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
