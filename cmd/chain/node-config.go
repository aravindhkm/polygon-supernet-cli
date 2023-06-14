package chain

import (
	"context"
	"fmt"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cli/cmd/config"
)

func CreateNodeConfigMap(nsArgs string) (string, error) {
	getParam, err := GetTotalNode(nsArgs)
	if err != nil {
		return "", err
	}

	totalNode, err := strconv.Atoi(getParam)

	for i := 1; i <= totalNode; i++ {
		var configMapName string = fmt.Sprintf("validator-node%v-config", i)
		configMapData := make(map[string]string)
		key := fmt.Sprintf("node%vconfig.json", i)
		configMapData[key] = fmt.Sprintf(
			`{
				"chain_config": "/data/genesis.json",
				"secrets_config": "/data/vaultsecretsconfig.json",
				"data_dir": "/data/node%v",
				"block_gas_target": "0x0",
				"grpc_addr": "0.0.0.0:9632",
				"jsonrpc_addr": "0.0.0.0:8545",
				"telemetry": {
					"prometheus_addr": "0.0.0.0:5001"
				},
				"network": {
					"no_discover": false,
					"libp2p_addr": "0.0.0.0:1478",
					"nat_addr": "",
					"dns_addr": "",
					"max_peers": -1,
					"max_outbound_peers": -1,
					"max_inbound_peers": -1
				},
				"seal": true,
				"tx_pool": {
					"price_limit": 0,
					"max_slots": 4096,
					"max_account_enqueued": 128
				},
				"log_level": "INFO",
				"restore_file": "",
				"headers": {
					"access_control_allow_origins": [
						"*"
					]
				},
				"log_to": "",
				"json_rpc_batch_request_limit": 20,
				"json_rpc_block_range_limit": 1000,
				"json_log_format": false,
				"relayer": false,
				"num_block_confirmations": 64
			}`, i)

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
			return "", err
		}
	}

	return "Validator node is successfully configured 📜", nil
}
