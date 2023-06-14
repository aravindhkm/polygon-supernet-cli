package chain

import (
	"context"
	"fmt"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"cli/cmd/config"
)

func CreateStorageClassAndPVC(nsArgs string) (string, error) {
	storage := &storagev1.StorageClass{
		AllowVolumeExpansion: toGetBooleanPtr(true),
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "polygonsc",
		},
		Parameters: map[string]string{
			"type": "pd-standard",
		},
		Provisioner:       "kubernetes.io/gce-pd",
		ReclaimPolicy:     toPVReclaimPolicyPtr("Delete"),
		VolumeBindingMode: toModePtr(storagev1.VolumeBindingWaitForFirstConsumer),
	}
	_, err := config.CLIENTSET.StorageV1().StorageClasses().Create(context.TODO(), storage, metav1.CreateOptions{})
	if err != nil {
		// response.StatusCode = http.StatusBadRequest
		// response.Success = false
		// response.Message = err.Error()
		// response.SendResponse(c)
		// return

		// return "", err
	}

	getParam, err := GetTotalNode(nsArgs)

	if err != nil {
		return "", err
	}

	totalNode, err := strconv.Atoi(getParam)

	for i := 1; i <= totalNode; i++ {
		fsMode := apiv1.PersistentVolumeFilesystem
		node := fmt.Sprintf("polygon-edge-validator-%v-pvc", i)

		validatorPVC := &apiv1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      node,
				Namespace: nsArgs,
			},
			Spec: apiv1.PersistentVolumeClaimSpec{
				AccessModes: []apiv1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				StorageClassName: toGetStringPtr("polygonsc"),
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						apiv1.ResourceName(apiv1.ResourceStorage): resource.MustParse("10Gi"),
					},
				},
				VolumeMode: &fsMode,
			},
		}
		_, err := config.CLIENTSET.CoreV1().PersistentVolumeClaims(nsArgs).Create(context.TODO(), validatorPVC, metav1.CreateOptions{})

		if err != nil {
			return "", err
		}
	}

	for i := 1; i < totalNode; i++ {
		node := fmt.Sprintf("validator-node%v-svc", i)

		servicePVC := &apiv1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      node,
				Namespace: nsArgs,
			},
			Spec: apiv1.ServiceSpec{
				Type: apiv1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"app":       "polygon-edge-network",
					"namespace": nsArgs,
				},
				Ports: []apiv1.ServicePort{
					{
						Name:     "grpc",
						Port:     9632,
						Protocol: apiv1.ProtocolTCP,
						TargetPort: intstr.IntOrString{
							IntVal: 9632,
						},
					},
					{
						Name:     "jsonrpc",
						Port:     8545,
						Protocol: apiv1.ProtocolTCP,
						TargetPort: intstr.IntOrString{
							IntVal: 8545,
						},
					},
					{
						Name:     "prometheus",
						Port:     5001,
						Protocol: apiv1.ProtocolTCP,
						TargetPort: intstr.IntOrString{
							IntVal: 5001,
						},
					},
					{
						Name:     "libp2p",
						Port:     1478,
						Protocol: apiv1.ProtocolTCP,
						TargetPort: intstr.IntOrString{
							IntVal: 1478,
						},
					},
				},
			},
		}
		_, err := config.CLIENTSET.CoreV1().Services(nsArgs).Create(context.TODO(), servicePVC, metav1.CreateOptions{})

		if err != nil {
			return "", err
		}
	}

	return "PersistentVolumeClaim & Validator Service is successfully configured 💾", nil
}

func toPVReclaimPolicyPtr(s string) *apiv1.PersistentVolumeReclaimPolicy {
	t := apiv1.PersistentVolumeReclaimPolicy(s)
	return &t
}

func toModePtr(m storagev1.VolumeBindingMode) *storagev1.VolumeBindingMode { return &m }

func toGetStringPtr(s string) *string { return &s }

func toGetBooleanPtr(s bool) *bool { return &s }
