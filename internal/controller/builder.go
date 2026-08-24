package controller

import (
	"fmt"
	"strings"

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

	defaultBuildpacksBuilderImage = "paketobuildpacks/builder-jammy-base:latest"
	buildpacksPackImage           = "buildpacksio/pack:0.40.6"
	buildpacksGitImage            = "alpine/git:2.47.2"
	buildpacksDockerImage         = "docker:29-dind"
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
	buildRun         *maxicloudv1alpha1.BuildRun
	jobName          string
	destination      string
	buildOutput      string
	sha              string
	repoSecretName   string
	owner            string
	repo             string
	registryInsecure bool
	packVolumeKey    string
}

func newBuildJob(params BuildJobParams) (*batchv1.Job, error) {
	strategy := maxicloudv1alpha1.BuildStrategyDockerfile
	if params.buildRun.Spec.Build != nil && params.buildRun.Spec.Build.Strategy != "" {
		strategy = params.buildRun.Spec.Build.Strategy
	}
	switch strategy {
	case maxicloudv1alpha1.BuildStrategyDockerfile:
		return newDockerfileBuildJob(params), nil
	case maxicloudv1alpha1.BuildStrategyBuildpacks:
		return newBuildpacksBuildJob(params), nil
	default:
		return nil, fmt.Errorf("unsupported build strategy: %s", strategy)
	}
}

// TODO: 非特権コンテナでビルドできるようにする
func newDockerfileBuildJob(params BuildJobParams) *batchv1.Job {
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
							Name:  "buildkit",
							Image: "moby/buildkit:latest",
							Env: []corev1.EnvVar{
								{
									Name: "GITHUB_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.repoSecretName},
											Key:                  installationAccessTokenKey,
										},
									},
								},
								{
									Name:  "XDG_RUNTIME_DIR",
									Value: "/tmp",
								},
								{
									Name:  "DOCKER_CONFIG",
									Value: "/root/.docker",
								},
							},
							Command: []string{
								"buildctl-daemonless.sh",
								"build",
								"--frontend=dockerfile.v0",
								"--opt", fmt.Sprintf("context=https://x-access-token:$(GITHUB_TOKEN)@github.com/%s/%s.git#%s", params.owner, params.repo, params.sha),
								"--opt", fmt.Sprintf("filename=%s", params.buildRun.Spec.Source.DockerfilePath),
								"--output", params.buildOutput,
							},
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

func newBuildpacksBuildJob(params BuildJobParams) *batchv1.Job {
	registryHost := strings.SplitN(params.destination, "/", 2)[0]
	packArgs := []string{
		"build", params.destination,
		"--path", "/workspace",
		"--builder", defaultBuildpacksBuilderImage,
		"--publish",
		"--trust-builder",
		"--network", "host",
	}
	if params.registryInsecure {
		packArgs = append(packArgs, "--insecure-registry", registryHost)
	}
	packEnv := []corev1.EnvVar{
		{Name: "DOCKER_HOST", Value: "unix:///var/run/docker/docker.sock"},
		{Name: "DOCKER_CONFIG", Value: "/root/.docker"},
	}
	if strings.TrimSpace(params.packVolumeKey) != "" {
		packEnv = append(packEnv, corev1.EnvVar{Name: "PACK_VOLUME_KEY", Value: params.packVolumeKey})
	}
	for _, env := range params.buildRun.Spec.Env {
		packEnv = append(packEnv, env)
		packArgs = append(packArgs, "--env", env.Name)
	}

	sidecarRestartPolicy := corev1.ContainerRestartPolicyAlways
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
					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: buildpacksGitImage,
							Env: []corev1.EnvVar{
								{
									Name: "GITHUB_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: params.repoSecretName},
											Key:                  installationAccessTokenKey,
										},
									},
								},
								{Name: "GIT_CONFIG_COUNT", Value: "1"},
								{Name: "GIT_CONFIG_KEY_0", Value: "http.https://github.com/.extraHeader"},
								{Name: "GIT_CONFIG_VALUE_0", Value: "Authorization: Bearer $(GITHUB_TOKEN)"},
							},
							Command: []string{"git"},
							Args: []string{
								"clone",
								"--no-checkout",
								fmt.Sprintf("https://github.com/%s/%s.git", params.owner, params.repo),
								"/workspace",
							},
							VolumeMounts: []corev1.VolumeMount{{Name: "source", MountPath: "/workspace"}},
						},
						{
							Name:         "git-checkout",
							Image:        buildpacksGitImage,
							Command:      []string{"git"},
							Args:         []string{"-C", "/workspace", "checkout", "--detach", params.sha},
							VolumeMounts: []corev1.VolumeMount{{Name: "source", MountPath: "/workspace"}},
						},
						{
							Name:          "docker",
							Image:         buildpacksDockerImage,
							RestartPolicy: &sidecarRestartPolicy,
							Args:          dockerDaemonArgs(registryHost, params.registryInsecure),
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{Command: []string{"docker", "info"}},
								},
								PeriodSeconds:    1,
								FailureThreshold: 60,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "docker-socket", MountPath: "/var/run/docker"},
								{Name: "docker-data", MountPath: "/var/lib/docker"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "buildpacks",
							Image: buildpacksPackImage,
							Args:  packArgs,
							Env:   packEnv,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "source", MountPath: "/workspace", ReadOnly: true},
								{Name: "docker-socket", MountPath: "/var/run/docker"},
								{Name: "registry-auth", MountPath: "/root/.docker", ReadOnly: true},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "source", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "docker-socket", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "docker-data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{
							Name: "registry-auth",
							VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
								SecretName: params.repoSecretName,
								Items:      []corev1.KeyToPath{{Key: corev1.DockerConfigJsonKey, Path: "config.json"}},
							}},
						},
					},
				},
			},
		},
	}
}

func dockerDaemonArgs(registryHost string, registryInsecure bool) []string {
	args := []string{
		"--host=unix:///var/run/docker/docker.sock",
		"--tls=false",
	}
	if registryInsecure {
		args = append(args, "--insecure-registry="+registryHost)
	}
	return args
}

func boolPtr(b bool) *bool { return &b }
