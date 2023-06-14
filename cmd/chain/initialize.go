package chain

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cli/cmd/config"

	"github.com/google/uuid"
)

type PremineAllo struct {
	Account string `json:"account"`
	Amount  string `json:"amount"`
}

type ConfigRequest struct {
	Name              string        `json:"name"`
	NumOfNodes        string        `json:"totalNode"`
	GasLimit          string        `json:"gasLimit"`
	EpochSize         string        `json:"epochSize"`
	NodePremineAmount string        `json:"nodePremineFund"`
	Premine           []PremineAllo `json:"premine"`
}

func CreateConfigMap(requestBody ConfigRequest) (string, string, error) {
	var nsArgs string = uuid.New().String()

	var passingArgs string = `command="polygon-edge genesis \`

	passingArgs = passingArgs + fmt.Sprintf("\n--block-gas-limit %s %s", requestBody.GasLimit, `\`)
	passingArgs = passingArgs + fmt.Sprintf("\n--epoch-size %s %s", requestBody.EpochSize, `\`)
	passingArgs = passingArgs + fmt.Sprintf("\n--name %s %s", requestBody.Name, `\`)
	passingArgs = passingArgs + fmt.Sprintf("\n--chain-id %s %s", "51001", `\`)
	passingArgs = passingArgs + fmt.Sprintf("\n--consensus %s %s", "ibft", `\`)

	for _, value := range requestBody.Premine {
		passingArgs = passingArgs + fmt.Sprintf("\n--premine %s:%s %s", value.Account, value.Amount, `\`)
	}

	passingArgs = passingArgs[0:len(passingArgs)-1] + fmt.Sprintf("%s", `"`)

	err := createNameSpace(nsArgs, requestBody.NumOfNodes)

	if err != nil {
		return "", "", err
	}

	err = createConfigMap(nsArgs)

	if err != nil {
		return "", "", err
	}

	err = createHelperJob(nsArgs, nsArgs, requestBody.NumOfNodes, passingArgs, requestBody.NodePremineAmount)

	if err != nil {
		return "", "", err
	}

	return nsArgs, "Initialize-Crypto is successfully configured 🔌", nil
}

func GetStakeId(nsArgs string) (string, error) {
	result, err := getStakeIdInfo(nsArgs)
	if err != nil {
		return "", err
	}

	return result, nil
}

func GetTotalNodeInNs(nsArgs string) (string, error) {
	result, err := GetTotalNode(nsArgs)
	if err != nil {
		return "", err
	}

	return result, nil
}

func GetJobDetails(nsArgs string) (map[string]any, error) {
	res, err := config.CLIENTSET.BatchV1().Jobs(nsArgs).Get(context.TODO(), "polygon-edge-job", metav1.GetOptions{})
	if err != nil {
		return map[string]any{}, err
	}

	var jsonMap []apiv1.EnvVar = res.Spec.Template.Spec.Containers[0].Env
	var store = map[string]any{}

	for _, value := range jsonMap {
		store[value.Name] = value.Value
	}

	return store, nil
}

func GetTotalNode(nsArgs string) (string, error) {
	job, err := config.CLIENTSET.CoreV1().Namespaces().Get(context.TODO(), nsArgs, metav1.GetOptions{})

	if err != nil {
		return "", err
	}

	return job.Labels["total-node"], nil
}

func createNameSpace(nsArgs string, totalNode string) error {
	namespace := &apiv1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nsArgs,
			Labels: map[string]string{
				"total-node": totalNode,
			},
		},
	}

	_, err := config.CLIENTSET.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	for {
		// Get the job
		job, err := config.CLIENTSET.CoreV1().Namespaces().Get(context.TODO(), nsArgs, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Check the job status
		if job.Status.Phase == "Active" {
			return nil
		} else if job.Status.Phase == "Terminating" {
			return fmt.Errorf("job failed")
		}

		time.Sleep(1 * time.Second)
	}
}

func createConfigMap(nsArgs string) error {
	var configMapName string = "vaultconfig-cm"
	configMapData := make(map[string]string)
	configMapData["vaultconfig-node.json"] = fmt.Sprintf(`{
		"token": "%s",
		"server_url": "%s",
		"type": "hashicorp-vault"
   	}`, config.VaultToken, config.VaultUrl)

	// Make ConfigMap
	configMap := &apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: nsArgs,
		},
		Data: configMapData,
	}
	_, err := config.CLIENTSET.CoreV1().ConfigMaps(nsArgs).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	} else {
		return nil
	}
}

func createHelperJob(nsArgs string, stackId string, node string, genesis string, nodePremineFund string) error {
	var jobName string = "polygon-edge-job"
	envs := []apiv1.EnvVar{
		{
			Name:  "NAMESPACE",
			Value: nsArgs,
		},
		{
			Name:  "NUM_OF_NODES",
			Value: node,
		},
		{
			Name:  "STACK_ID",
			Value: stackId,
		},
		{
			Name:  "PREMINE_FUND",
			Value: nodePremineFund,
		},
		{
			Name:  "VAULT_ADDR",
			Value: config.VaultUrl,
		},
		{
			Name:  "VAULT_TOKEN",
			Value: config.VaultToken,
		},
	}

	jobSpec := batchv1.JobSpec{
		Template: apiv1.PodTemplateSpec{
			Spec: apiv1.PodSpec{
				RestartPolicy: "OnFailure",
				Volumes: []apiv1.Volume{
					{
						Name: "vault-configcm",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "vaultconfig-cm",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:    jobName,
						Image:   "0xpolygon/polygon-edge:0.9.0",
						Command: []string{"/bin/sh", "-c"},
						Env:     envs,
						VolumeMounts: []apiv1.VolumeMount{
							{
								Name:      "vault-configcm",
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
				  
							for i in $(seq 1 $((NUM_OF_NODES)));
							do 
							  token=$(jq -r .token /config/vaultconfig-node.json)
							  server_url=$(jq -r .server_url /config/vaultconfig-node.json)
							  type=$(jq -r .type /config/vaultconfig-node.json)
							  name=${STACK_ID}/node${i}
							  echo "{\"token\": \"$token\", \"server_url\": \"$server_url\", \"type\": \"$type\", \"name\": \"$name\"}" > /home/vaultconfignode${i}.json
							  polygon-edge secrets init --config /home/vaultconfignode${i}.json --json | jq > /home/node${i}keys.json
							done				  
							
							%s

							for i in $(seq 1 $((NUM_OF_NODES))); 
							do
							  address=$(jq -r '.[].address' /home/node${i}keys.json)
							  bls_pubkey=$(jq -r '.[].bls_pubkey' /home/node${i}keys.json)
							  command=${command}"--ibft-validator ${address}:${bls_pubkey} "
							done 
				  
							for i in $(seq 1 $((NUM_OF_NODES))); 
							do
							  address=$(jq -r '.[].address' /home/node${i}keys.json)
							  command=${command}"--premine=${address}:${PREMINE_FUND} "
							done
				  
							for i in $(seq 1 $((NUM_OF_NODES))); 
							do
							  node_id=$(jq -r '.[].node_id' /home/node${i}keys.json)
							  command=${command}"--bootnode /dns4/validator-node${i}-svc.${NAMESPACE}.svc.cluster.local/tcp/1478/p2p/${node_id} "
							done
				  
							# echo $command
							eval "$command"
				  
							# Set vault variables
							set -e
				  
							SECRET_PATH="polygon-edge/data/${STACK_ID}/genesis.json"
							JSON_FILE_PATH="/genesis.json"
				  
							# Read JSON file contents into a variable
							JSON_CONTENTS="$(cat $JSON_FILE_PATH)"
				  
							# Create the secret in Vault
							
							curl --header "X-Vault-Token: ${VAULT_TOKEN}" \
							--request POST \
							--data "{\"data\": ${JSON_CONTENTS}}" \
							${VAULT_ADDR}/v1/${SECRET_PATH}
				  
							echo "Secret successfully written genesis.json to Vault!"
				  
							# Writing public keys to vault in json format
				  
							SECRET_PATH="polygon-edge/data/${STACK_ID}"
				  
							for i in $(seq 1 $((NUM_OF_NODES))); 
							do
							  PUBKEYS_JSON_FILE_PATH="/home/node${i}keys.json"
							  PUBKEYS_JSON_CONTENTS="$(cat $PUBKEYS_JSON_FILE_PATH)"
							  curl --header "X-Vault-Token: ${VAULT_TOKEN}" \
							  --request POST \
							  --data "{\"data\": ${PUBKEYS_JSON_CONTENTS}}" \
							  ${VAULT_ADDR}/v1/${SECRET_PATH}/node${i}/keys.json
				  
							  echo "Secret successfully written node ${i} keys json to Vault!"
							done 
				  
							# Writing vault secrets config to vault
				  
							VAULT_SECRET_PATH="polygon-edge/data/${STACK_ID}"
				  
							for i in $(seq 1 $((NUM_OF_NODES))); 
							do
							  VAULTCONFIG_JSON_FILE_PATH="/home/vaultconfignode${i}.json"
							  VAULTCONFIG_JSON_CONTENTS="$(cat $VAULTCONFIG_JSON_FILE_PATH)"
							  curl --header "X-Vault-Token: ${VAULT_TOKEN}" \
							  --request POST \
							  --data "{\"data\": ${VAULTCONFIG_JSON_CONTENTS}}" \
							  ${VAULT_ADDR}/v1/${VAULT_SECRET_PATH}/node${i}/vaultsecretsconfig.json
				  
							  echo "Secret successfully written node ${i} vault secrets config json to Vault!"
							done 
				  	`, genesis),
						},
					},
				},
			},
		},
	}

	// Make ConfigMap
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: nsArgs,
		},
		Spec: jobSpec,
	}
	_, err := config.CLIENTSET.BatchV1().Jobs(nsArgs).Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	for {
		// Get the job
		job, err := config.CLIENTSET.BatchV1().Jobs(nsArgs).Get(context.TODO(), jobName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Check the job status
		if job.Status.Succeeded == 1 {
			return nil
		} else if job.Status.Failed > 0 {
			return fmt.Errorf("job failed")
		}

		time.Sleep(1 * time.Second)
	}
}

func getStakeIdInfo(nsArgs string) (string, error) {
	res, err := config.CLIENTSET.BatchV1().Jobs(nsArgs).Get(context.TODO(), "polygon-edge-job", metav1.GetOptions{})

	if err != nil {
		return "", err
	}

	var jsonMap []apiv1.EnvVar = res.Spec.Template.Spec.Containers[0].Env
	var result string

	for _, value := range jsonMap {
		if value.Name == "STACK_ID" {
			result = value.Value
		}
	}

	return result, nil
}
