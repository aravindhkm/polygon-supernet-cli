package config

import (
	"flag"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var CLIENTSET *kubernetes.Clientset

var isK8is bool = false
var VaultToken string = ""
var VaultUrl string = ""

func InitConfig() {
	CLIENTSET = authK8s()
}

func authK8s() *kubernetes.Clientset {
	var config *rest.Config
	var err error

	if false {
		// log.Println("****inside ONK8S****")
		config, err = rest.InClusterConfig()
	} else {
		// log.Println("****inside Kuber config****")
		if flag.Lookup("kubeconfig") != nil {
			var kconfig string = flag.Lookup("kubeconfig").Value.String()
			config, err = clientcmd.BuildConfigFromFlags("", kconfig)
		} else {
			kubeconfig := flag.String("config", "kubeconfig", "Kubeconfig file")
			// log.Println("***found kubeconfig***")
			flag.Parse()
			config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		}
	}

	if err != nil {
		log.Panicf("Error while building config %s", err.Error())
	}

	CLIENTSET, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln("Error while creating K8 client", err.Error())
	}	

	return CLIENTSET
}
