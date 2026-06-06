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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentPipelinePhase string

const (
	DeploymentPipelinePhaseQueued    DeploymentPipelinePhase = "Queued"
	DeploymentPipelinePhaseBuilding  DeploymentPipelinePhase = "Building"
	DeploymentPipelinePhaseDeploying DeploymentPipelinePhase = "Deploying"
	DeploymentPipelinePhaseSucceeded DeploymentPipelinePhase = "Succeeded"
	DeploymentPipelinePhaseFailed    DeploymentPipelinePhase = "Failed"
)

// DeploymentPipelineSpec defines the desired state of DeploymentPipeline
type DeploymentPipelineSpec struct {
	// ApplicationName is the name of the Application CR to deploy.
	// +required
	ApplicationName string `json:"applicationName"`

	// Owner is the GitHub repository owner (org or user).
	// +required
	Owner string `json:"owner"`

	// Repo is the GitHub repository name.
	// +required
	Repo string `json:"repo"`

	// SHA is the commit SHA to build and deploy.
	// +required
	SHA string `json:"sha"`

	// PRNumber is the pull request number. Set only for PR-triggered deployments.
	// +optional
	PRNumber *int `json:"prNumber,omitempty"`
}

// DeploymentPipelineStatus defines the observed state of DeploymentPipeline.
type DeploymentPipelineStatus struct {
	// Phase is the current lifecycle phase of the pipeline.
	// +kubebuilder:validation:Enum=Queued;Building;Deploying;Succeeded;Failed
	// +optional
	Phase DeploymentPipelinePhase `json:"phase,omitempty"`

	// CheckRunID is the GitHub Check Run ID created for this pipeline.
	// +optional
	CheckRunID int64 `json:"checkRunID,omitempty"`

	// DeploymentSummaryCommentID is the GitHub PR comment ID created for this pipeline.
	// +optional
	DeploymentSummaryCommentID int64 `json:"deploymentSummaryCommentID,omitempty"`

	// BuildRunRef is the name of the BuildRun CR created for this pipeline.
	// +optional
	BuildRunRef string `json:"buildRunRef,omitempty"`

	// Image is the built container image reference, set after build succeeds.
	// +optional
	Image string `json:"image,omitempty"`

	// StartedAt is when the pipeline started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// FinishedAt is when the pipeline completed.
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`

	// Conditions contains the conditions for the DeploymentPipeline.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="SHA",type="string",JSONPath=".spec.sha"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DeploymentPipeline is the Schema for the deploymentpipelines API
type DeploymentPipeline struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec DeploymentPipelineSpec `json:"spec"`

	// +optional
	Status DeploymentPipelineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DeploymentPipelineList contains a list of DeploymentPipeline
type DeploymentPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []DeploymentPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeploymentPipeline{}, &DeploymentPipelineList{})
}
