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

type BuildStrategy string

const (
	BuildStrategyDockerfile BuildStrategy = "Dockerfile"
	BuildStrategyBuildpacks BuildStrategy = "Buildpacks"
)

type DockerfileSource string

const (
	DockerfileSourcePath   DockerfileSource = "Path"
	DockerfileSourceInline DockerfileSource = "Inline"
)

// DockerfileBuildConfig defines how the Dockerfile is supplied.
type DockerfileBuildConfig struct {
	// Source selects whether Path or Inline is used.
	// +kubebuilder:validation:Enum=Path;Inline
	// +required
	Source DockerfileSource `json:"source"`

	// Path is the path to the Dockerfile in the repository.
	// +optional
	Path string `json:"path,omitempty"`

	// Inline is the Dockerfile content supplied by the user.
	// +optional
	Inline string `json:"inline,omitempty"`
}

// BuildpacksBuildConfig defines Cloud Native Buildpacks settings.
type BuildpacksBuildConfig struct {
	// Builder is the builder image. The platform default is used when empty.
	// +optional
	Builder string `json:"builder,omitempty"`
}

// BuildConfig is a snapshot of the user-selected build configuration.
type BuildConfig struct {
	// Strategy selects the image build implementation.
	// +kubebuilder:validation:Enum=Dockerfile;Buildpacks
	// +required
	Strategy BuildStrategy `json:"strategy"`

	// Dockerfile contains settings for a Dockerfile build.
	// +optional
	Dockerfile *DockerfileBuildConfig `json:"dockerfile,omitempty"`

	// Buildpacks contains settings for a Cloud Native Buildpacks build.
	// +optional
	Buildpacks *BuildpacksBuildConfig `json:"buildpacks,omitempty"`
}

// BuildSource defines Git source information used for a build.
type BuildSource struct {
	// Repo is the URL of the Git repository.
	// +required
	RepoURL string `json:"repo"`

	// SHA is the commit SHA to build. If empty, the latest branch commit is used.
	// +optional
	SHA string `json:"sha,omitempty"`

	// DockerfilePath is the path to the Dockerfile in the repository.
	// +optional
	// +kubebuilder:default=./Dockerfile
	DockerfilePath string `json:"dockerfilePath,omitempty"`
}

// BuildRunSpec defines the desired state of BuildRun.
// BuildRun is expected to be created by the API server or controllers, not directly by end users.
type BuildRunSpec struct {
	// Source is the build source snapshot.
	// +required
	Source BuildSource `json:"source"`

	// Build is the build configuration snapshot.
	// +optional
	Build *BuildConfig `json:"build,omitempty"`

	// Env is a list of build-time environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

type BuildRunPhase string

const (
	BuildRunPhaseQueued    BuildRunPhase = "Queued"
	BuildRunPhaseBuilding  BuildRunPhase = "Building"
	BuildRunPhasePushing   BuildRunPhase = "Pushing"
	BuildRunPhaseSucceeded BuildRunPhase = "Succeeded"
	BuildRunPhaseFailed    BuildRunPhase = "Failed"
	BuildRunPhaseCanceled  BuildRunPhase = "Canceled"
)

// BuildRunStatus defines the observed state of BuildRun.
type BuildRunStatus struct {
	// Phase represents the current lifecycle phase.
	// +kubebuilder:validation:Enum=Queued;Building;Pushing;Succeeded;Failed;Canceled
	// +optional
	Phase BuildRunPhase `json:"phase,omitempty"`

	// Image is the resulting image reference on success.
	// +optional
	Image string `json:"image,omitempty"`

	// StartedAt is when the build execution started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the build execution completed.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// Conditions represent the current state of the BuildRun resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".status.image"

// BuildRun is the Schema for the buildruns API
type BuildRun struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of BuildRun
	// +required
	Spec BuildRunSpec `json:"spec"`

	// status defines the observed state of BuildRun
	// +optional
	Status BuildRunStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// BuildRunList contains a list of BuildRun
type BuildRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []BuildRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BuildRun{}, &BuildRunList{})
}
