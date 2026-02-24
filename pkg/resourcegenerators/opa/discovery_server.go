package opa

import (
	"fmt"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	opaDiscoveryContainerName              = "opa-discovery"
	opaDiscoveryFetcherContainerName       = "opa-discovery-bundle-fetcher"
	opaDiscoveryContainerPort        int32 = 8080
	opaDiscoveryServicePort          int32 = 80
	// Keep the resource path stable to avoid requiring OPA sidecar restarts during migration.
	opaDiscoveryPath = "/discovery.tar.gz"
	opaBundlePath    = "/bundles/authz.tar.gz"

	opaDiscoveryNginxConfigMountPath  = "/etc/nginx/conf.d"
	opaDiscoverySecretMountPath       = "/var/run/accesserator/opa-secret"
	opaDiscoveryPublicKeyMountPath    = "/var/run/accesserator/opa-public-key"
	opaDiscoveryBundleMountPath       = "/var/run/accesserator/bundles"
	opaDiscoveryGithubTokenFile       = "github-token"
	opaDiscoveryPublicKeyFile         = "public.pem"
	opaDiscoveryMirroredBundleFile    = "authz.tar.gz"
	opaDiscoveryBundleRefreshInterval = "1m"
	opaDiscoveryFetcherHeartbeatFile  = ".fetcher-heartbeat"
	opaDiscoveryFetcherLivenessMaxAge = "3m"
)

func GetDiscoveryConfigDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	discoveryDocument := DiscoveryDocument{
		Bundles: map[string]Bundle{
			"authz": {
				Service:  "discovery-server",
				Resource: GetOpaDiscoveryBundleResourcePath(),
				Polling: Polling{
					MinDelaySeconds: 10,
					MaxDelaySeconds: 30,
				},
			},
		},
	}

	discoveryBundle, err := buildDiscoveryBundleArchive(discoveryDocument)
	if err != nil {
		return nil
	}

	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			utilities.OpaDiscoveryNginxConfFileName: renderDiscoveryNginxConf(),
		},
		BinaryData: map[string][]byte{
			utilities.OpaDiscoveryBundleFileName: discoveryBundle,
		},
	}
}

func GetDiscoveryServiceDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *corev1.Service {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":      objectMeta.Name,
		"app.kubernetes.io/component": opaDiscoveryContainerName,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectMeta.Name,
			Namespace: objectMeta.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       opaDiscoveryServicePort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(opaDiscoveryContainerPort),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func GetDiscoveryDeploymentDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *appsv1.Deployment {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	labels := map[string]string{
		"app.kubernetes.io/name":      utilities.GetOpaDiscoveryServiceName(scope.SecurityConfig.Spec.ApplicationRef),
		"app.kubernetes.io/component": opaDiscoveryContainerName,
	}

	discoveryConfigName := utilities.GetOpaDiscoveryConfigName(scope.SecurityConfig.Spec.ApplicationRef)
	tokenFilePath := fmt.Sprintf("%s/%s", opaDiscoverySecretMountPath, opaDiscoveryGithubTokenFile)
	publicKeyFilePath := fmt.Sprintf("%s/%s", opaDiscoveryPublicKeyMountPath, opaDiscoveryPublicKeyFile)
	mirroredBundleFilePath := getOpaDiscoveryMirroredBundleFilePath()
	fetcherHeartbeatFilePath := getOpaDiscoveryFetcherHeartbeatFilePath()
	fetcherImage := fmt.Sprintf("%s:%s", config.Get().AccesseratorImageName, config.Get().AccesseratorImageTag)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectMeta.Name,
			Namespace: objectMeta.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: utilities.Ptr[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  opaDiscoveryContainerName,
							Image: "nginxinc/nginx-unprivileged:latest",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: opaDiscoveryContainerPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "discovery",
									MountPath: opaDiscoveryNginxConfigMountPath,
									ReadOnly:  true,
								},
								{
									Name:      "mirrored-bundle",
									MountPath: opaDiscoveryBundleMountPath,
									ReadOnly:  true,
								},
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: GetOpaDiscoveryBundleResourcePath(),
										Port: intstr.FromString("http"),
									},
								},
								PeriodSeconds:    2,
								FailureThreshold: 30,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: GetOpaDiscoveryBundleResourcePath(),
										Port: intstr.FromString("http"),
									},
								},
								PeriodSeconds: 5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: GetOpaDiscoveryResourcePath(),
										Port: intstr.FromString("http"),
									},
								},
								PeriodSeconds: 10,
							},
						},
						{
							Name:            opaDiscoveryFetcherContainerName,
							Image:           fetcherImage,
							ImagePullPolicy: getOpaDiscoveryFetcherImagePullPolicy(),
							Command:         []string{"/opa-discovery-fetcher"},
							Args: []string{
								"-bundle-ref=" + scope.OpaConfig.BundleUrl,
								"-github-token-file=" + tokenFilePath,
								"-public-key-file=" + publicKeyFilePath,
								"-output-file=" + mirroredBundleFilePath,
								"-refresh-interval=" + opaDiscoveryBundleRefreshInterval,
								"-heartbeat-file=" + fetcherHeartbeatFilePath,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "github-token",
									MountPath: opaDiscoverySecretMountPath,
									ReadOnly:  true,
								},
								{
									Name:      "bundle-public-key",
									MountPath: opaDiscoveryPublicKeyMountPath,
									ReadOnly:  true,
								},
								{
									Name:      "mirrored-bundle",
									MountPath: opaDiscoveryBundleMountPath,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/opa-discovery-fetcher",
											"-healthcheck-heartbeat-file=" + fetcherHeartbeatFilePath,
											"-healthcheck-max-age=" + opaDiscoveryFetcherLivenessMaxAge,
										},
									},
								},
								PeriodSeconds: 10,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "discovery",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: discoveryConfigName,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  utilities.OpaDiscoveryBundleFileName,
											Path: utilities.OpaDiscoveryBundleFileName,
										},
										{
											Key:  utilities.OpaDiscoveryNginxConfFileName,
											Path: utilities.OpaDiscoveryNginxConfFileName,
										},
									},
								},
							},
						},
						{
							Name: "github-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: scope.SecurityConfig.Spec.Opa.GithubToken.Name,
									Items: []corev1.KeyToPath{
										{
											Key:  scope.SecurityConfig.Spec.Opa.GithubToken.Key,
											Path: opaDiscoveryGithubTokenFile,
										},
									},
								},
							},
						},
						{
							Name: "bundle-public-key",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: scope.SecurityConfig.Spec.Opa.BundlePublicKey.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  scope.SecurityConfig.Spec.Opa.BundlePublicKey.Key,
											Path: opaDiscoveryPublicKeyFile,
										},
									},
								},
							},
						},
						{
							Name: "mirrored-bundle",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func GetOpaDiscoveryResourcePath() string {
	return opaDiscoveryPath
}

func GetOpaDiscoveryBundleResourcePath() string {
	return opaBundlePath
}

func getOpaDiscoveryMirroredBundleFilePath() string {
	return fmt.Sprintf("%s/%s", opaDiscoveryBundleMountPath, opaDiscoveryMirroredBundleFile)
}

func getOpaDiscoveryFetcherHeartbeatFilePath() string {
	return fmt.Sprintf("%s/%s", opaDiscoveryBundleMountPath, opaDiscoveryFetcherHeartbeatFile)
}

func getOpaDiscoveryFetcherImagePullPolicy() corev1.PullPolicy {
	policy := corev1.PullPolicy(config.Get().AccesseratorImagePullPolicy)
	switch policy {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		return policy
	default:
		return corev1.PullNever
	}
}
