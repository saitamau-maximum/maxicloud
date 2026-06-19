package controller

import (
	"fmt"

	maxicloudv1alpha1 "github.com/saitamau-maximum/maxicloud/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ApplicationCRKind = "Application"
	BuildRunCRKind    = "BuildRun"
)

func newAppRegistrySecret(app *maxicloudv1alpha1.Application, dockerConfig string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appRegistrySecretName(app.Name),
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, maxicloudv1alpha1.GroupVersion.WithKind(ApplicationCRKind)),
			},
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(dockerConfig),
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
}

func newDeployment(app *maxicloudv1alpha1.Application, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, maxicloudv1alpha1.GroupVersion.WithKind(ApplicationCRKind)),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{appNameLabel: app.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{appNameLabel: app.Name},
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: appRegistrySecretName(app.Name)},
					},
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: image,
							Env:   app.Spec.Env,
						},
					},
				},
			},
		},
	}
}

func newService(app *maxicloudv1alpha1.Application) *corev1.Service {
	port := app.Spec.Expose.Port
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, maxicloudv1alpha1.GroupVersion.WithKind(ApplicationCRKind)),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{appNameLabel: app.Name},
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: intstr.FromInt32(port),
				},
			},
		},
	}
}

func newIngress(app *maxicloudv1alpha1.Application, baseDomain, ingressClassName string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	port := app.Spec.Expose.Port
	host := app.Spec.Expose.Domain
	if host == "" {
		host = baseDomain
	}
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, maxicloudv1alpha1.GroupVersion.WithKind(ApplicationCRKind)),
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: app.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newBuildRunSecret(buildRun *maxicloudv1alpha1.BuildRun, dockerConfig, installationAccessToken string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildRun.Name,
			Namespace: buildRun.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(buildRun, maxicloudv1alpha1.GroupVersion.WithKind(BuildRunCRKind)),
			},
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(dockerConfig),
			installationAccessTokenKey: []byte(installationAccessToken),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

type BuildJobParams struct {
	buildRun       *maxicloudv1alpha1.BuildRun
	jobName        string
	destination    string
	buildOutput    string
	sha            string
	repoSecretName string
	owner          string
	repo           string
}

// TODO: 非特権コンテナでビルドできるようにする
func newBuildJob(params BuildJobParams) *batchv1.Job {
	dockerfilePath, dockerfileInline := dockerfileConfig(params.buildRun)
	command := []string{
		"buildctl-daemonless.sh",
		"build",
		"--frontend=dockerfile.v0",
		"--opt", fmt.Sprintf("context=https://x-access-token:$(GITHUB_TOKEN)@github.com/%s/%s.git#%s", params.owner, params.repo, params.sha),
		"--opt", fmt.Sprintf("filename=%s", dockerfilePath),
		"--output", params.buildOutput,
	}
	containerEnv := []corev1.EnvVar{
		{
			Name: "GITHUB_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: params.repoSecretName},
					Key:                  installationAccessTokenKey,
				},
			},
		},
		{Name: "XDG_RUNTIME_DIR", Value: "/tmp"},
		{Name: "DOCKER_CONFIG", Value: "/root/.docker"},
	}
	if dockerfileInline != "" {
		containerEnv = append(containerEnv, corev1.EnvVar{Name: "DOCKERFILE_INLINE", Value: dockerfileInline})
		command = []string{
			"sh", "-c",
			`mkdir -p /tmp/dockerfile && printf '%s' "$DOCKERFILE_INLINE" > /tmp/dockerfile/Dockerfile && exec buildctl-daemonless.sh build --frontend=dockerfile.v0 --opt "context=https://x-access-token:${GITHUB_TOKEN}@github.com/` + params.owner + `/` + params.repo + `.git#` + params.sha + `" --local dockerfile=/tmp/dockerfile --opt filename=Dockerfile --output "` + params.buildOutput + `"`,
		}
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.jobName,
			Namespace: params.buildRun.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(params.buildRun, maxicloudv1alpha1.GroupVersion.WithKind(BuildRunCRKind)),
			},
			Labels: params.buildRun.Labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "buildkit",
							Image:   "moby/buildkit:latest",
							Env:     containerEnv,
							Command: command,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
								SeccompProfile: &corev1.SeccompProfile{
									Type: corev1.SeccompProfileTypeUnconfined,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "registry-auth",
									MountPath: "/root/.docker",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "registry-auth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: params.repoSecretName,
									Items: []corev1.KeyToPath{
										{
											Key:  corev1.DockerConfigJsonKey,
											Path: "config.json",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func dockerfileConfig(buildRun *maxicloudv1alpha1.BuildRun) (path, inline string) {
	path = buildRun.Spec.Source.DockerfilePath
	if path == "" {
		path = "./Dockerfile"
	}
	if buildRun.Spec.Build == nil || buildRun.Spec.Build.Dockerfile == nil {
		return path, ""
	}
	dockerfile := buildRun.Spec.Build.Dockerfile
	switch dockerfile.Source {
	case maxicloudv1alpha1.DockerfileSourcePath:
		if dockerfile.Path != "" {
			path = dockerfile.Path
		}
	case maxicloudv1alpha1.DockerfileSourceInline:
		inline = dockerfile.Inline
	}
	return path, inline
}

func boolPtr(b bool) *bool { return &b }
