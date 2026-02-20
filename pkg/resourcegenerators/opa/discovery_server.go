package opa

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"

	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	opaDiscoveryContainerName = "opa-discovery"
	opaDiscoveryContainerPort = int32(8080)
	opaDiscoveryServicePort   = int32(80)
	// Keep the resource path stable to avoid requiring OPA sidecar restarts during migration.
	opaDiscoveryPath = "/discovery.json"
)

type DiscoveryDocument struct {
	Bundles map[string]Bundle `json:"bundles"`
}

func GetDiscoveryConfigDesired(objectMeta metav1.ObjectMeta, scope state.Scope) *corev1.ConfigMap {
	if !scope.OpaConfig.Enabled {
		return nil
	}

	discoveryDocument := DiscoveryDocument{
		Bundles: map[string]Bundle{
			"authz": {
				Service:  "ghcr-registry",
				Resource: scope.OpaConfig.BundleUrl,
				Polling: Polling{
					MinDelaySeconds: 10,
					MaxDelaySeconds: 30,
				},
				Signing: Signing{
					KeyID: "bundle-verification-key",
				},
			},
		},
	}

	discoveryBundle, err := createDiscoveryBundle(discoveryDocument)
	if err != nil {
		return nil
	}

	return &corev1.ConfigMap{
		ObjectMeta: objectMeta,
		Data: map[string]string{
			utilities.OpaDiscoveryNginxConfFileName: fmt.Sprintf(`server {
  listen %d;
  server_name _;

  location = %s {
    alias /etc/nginx/conf.d/%s;
    default_type application/gzip;
    add_header Cache-Control "no-store";
  }
}
`, opaDiscoveryContainerPort, opaDiscoveryPath, utilities.OpaDiscoveryBundleFileName),
		},
		BinaryData: map[string][]byte{
			utilities.OpaDiscoveryBundleFileName: discoveryBundle,
		},
	}
}

func createDiscoveryBundle(discoveryDocument DiscoveryDocument) ([]byte, error) {
	// Discovery expects an OPA bundle archive (tar.gz). The bundle data document
	// contains dynamic bundle configuration that OPA merges into runtime config.
	discoveryDataJSON, err := json.Marshal(discoveryDocument)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "data.json",
		Mode: 0o644,
		Size: int64(len(discoveryDataJSON)),
	}); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}

	if _, err := tarWriter.Write(discoveryDataJSON); err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return nil, err
	}

	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
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
									MountPath: "/etc/nginx/conf.d",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "discovery",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: utilities.GetOpaDiscoveryConfigName(scope.SecurityConfig.Spec.ApplicationRef),
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
					},
				},
			},
		},
	}
}

func GetOpaDiscoveryResourcePath() string {
	return opaDiscoveryPath
}
