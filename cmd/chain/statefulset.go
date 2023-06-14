package chain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"cli/cmd/config"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

func CreateStateFulSet(nsArgs string) (string, error) {
	getParam, err := GetTotalNode(nsArgs)

	if err != nil {
		return "", err
	}

	totalNode, err := strconv.Atoi(getParam)

	stackId, err := getStakeIdInfo(nsArgs)

	if err != nil {
		return "", err
	}

	for i := 1; i <= totalNode; i++ {
		var replicas int32 = 1
		var jobName string = fmt.Sprintf("validator-node-%v", i)

		jobSpec := appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: jobName,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":       "polygon-edge-network",
					"namespace": nsArgs,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":       "polygon-edge-network",
						"namespace": nsArgs,
					},
				},
				Spec: apiv1.PodSpec{
					InitContainers: []apiv1.Container{
						{
							Name:  "fetch-from-vault",
							Image: "alpine:latest",
							Env: []apiv1.EnvVar{
								{
									Name:  "STACK_ID",
									Value: stackId,
								},
								{
									Name:  "VAULT_ADDR",
									Value: config.VaultUrl,
								},
								{
									Name:  "VAULT_TOKEN",
									Value: config.VaultToken,
								},
							},
							Command: []string{"sh", "-c"},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      fmt.Sprintf("data-validator-node%v", i),
									MountPath: "/data",
								},
								{
									Name:      "config-json",
									MountPath: "/config",
								},
							},
							Args: []string{
								fmt.Sprintf(
									`         
									#!/usr/bin/env sh
									# Install jq and curl
									apk add --no-cache jq
									apk add curl
									# Set vault variables
									set -e
									
									SECRET_PATH="polygon-edge/data/${STACK_ID}/genesis.json"
									JSON_FILE_PATH="/data/genesis.json"
						
									curl --header "X-Vault-Token: ${VAULT_TOKEN}" \
									${VAULT_ADDR}/v1/${SECRET_PATH} | jq -r '.data.data' > ${JSON_FILE_PATH}
						
									ls -lrt /data
						
									cat /data/genesis.json
						
									VAULTCONFIG_SECRET_PATH="polygon-edge/data/${STACK_ID}/node%v/vaultsecretsconfig.json"
									VAULTCONFIG_JSON_FILE_PATH="/data/vaultsecretsconfig.json"
									curl --header "X-Vault-Token: ${VAULT_TOKEN}" \
									${VAULT_ADDR}/v1/${VAULTCONFIG_SECRET_PATH} | jq -r '.data.data' > ${VAULTCONFIG_JSON_FILE_PATH}
								  `, i),
							},
						},
					},
					Volumes: []apiv1.Volume{
						{
							Name: fmt.Sprintf("data-validator-node%v", i),
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("polygon-edge-validator-%v-pvc", i),
								},
							},
						},
						{
							Name: "config-json",
							VolumeSource: apiv1.VolumeSource{
								ConfigMap: &apiv1.ConfigMapVolumeSource{
									LocalObjectReference: apiv1.LocalObjectReference{
										Name: fmt.Sprintf("validator-node%v-config", i),
									},
								},
							},
						},
					},
					Containers: []apiv1.Container{
						{
							Name:    jobName,
							Image:   "0xpolygon/polygon-edge:0.9.0",
							Command: []string{"sh", "-c"},
							Args: []string{
								fmt.Sprintf(`         
									echo "Executing"
									polygon-edge server --config /config/node%vconfig.json
								  `, i),
							},
							Ports: []apiv1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: 9632,
								},
								{
									Name:          "jsonrpc",
									ContainerPort: 8545,
								},
								{
									Name:          "prometheus",
									ContainerPort: 5001,
								},
								{
									Name:          "libp2p",
									ContainerPort: 1478,
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								{
									Name:      fmt.Sprintf("data-validator-node%v", i),
									MountPath: "/data",
								},
								{
									Name:      "config-json",
									MountPath: "/config",
								},
							},
						},
					},
				},
			},
		}

		// Make ConfigMap
		sts := &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: nsArgs,
				Labels: map[string]string{
					"app":       "polygon-edge-network",
					"namespace": nsArgs,
				},
			},
			Spec: jobSpec,
		}
		_, err := config.CLIENTSET.AppsV1().StatefulSets(nsArgs).Create(context.TODO(), sts, metav1.CreateOptions{})
		if err != nil {
			return "", err
		}
	}

	pi := createPodInformer()
	labelMap, err := metav1.LabelSelectorAsMap(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app":       "polygon-edge-network",
			"namespace": nsArgs,
		},
	})
	if err != nil {
		return "", err
	}

	var podsStatusCount int
	var podsStatus = map[string]bool{}

	for {
		pods, err := pi.Lister().List(labels.SelectorFromSet(labelMap))
		if err != nil {
			return "", err
		}

		if len(pods) == 0 {
			continue
		}

		for _, value := range pods {
			if value.Status.Phase == "Running" {
				if !podsStatus[value.Name] {
					podsStatus[value.Name] = true
					podsStatusCount++
				}
			}

			if value.Status.Phase == "Failed" {
				return "", err
			}
		}

		if totalNode == podsStatusCount {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return "Statefulset is successfully configured 🕹️", nil
}

func createPodInformer() v1.PodInformer {
	informerFactory := informers.NewSharedInformerFactory(config.CLIENTSET, time.Second*30)
	podInfoermer := informerFactory.Core().V1().Pods()
	podInfoermer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			// AddFunc: func(new interface{}) {
			// 	log.Println(new)
			// },
			UpdateFunc: func(old, new interface{}) {
				_ = new.(*apiv1.Pod)
				// log.Println(pod.Status.Phase)
			},
			// DeleteFunc: func(obj interface{}) {
			// 	log.Println(obj)
			// },
		},
	)
	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)
	return podInfoermer
}

func CreateLoadBalancer(nsArgs string) (string, error) {
	servicePVC := &apiv1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polygon-edge-svc",
			Namespace: nsArgs,
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeLoadBalancer,
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

	var ip_addr string
	for {
		res, err := config.CLIENTSET.CoreV1().Services(nsArgs).Get(context.TODO(), "polygon-edge-svc", metav1.GetOptions{})

		if err != nil {
			return "", err
		}

		if len(res.Status.LoadBalancer.Ingress) > 0 {
			ip_addr = res.Status.LoadBalancer.Ingress[0].IP
			break
		}
		time.Sleep(1 * time.Second)
	}

	if ip_addr != "" {
		return "LoadBalancer is successfully configured 📦", nil
	} else {
		return ip_addr, errors.New("LoadBalancer config is failed")
	}
}

func GetLoadBalancerInfo(nsArgs string) (string, error) {
	res, err := config.CLIENTSET.CoreV1().Services(nsArgs).Get(context.TODO(), "polygon-edge-svc", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var ip_addr string
	if len(res.Status.LoadBalancer.Ingress) > 0 {
		ip_addr = res.Status.LoadBalancer.Ingress[0].IP
	}

	_ = ip_addr

	return ip_addr, nil
}
