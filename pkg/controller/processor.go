package controller

import (
	"context"
	"fmt"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/insights-operator/pkg/config"
)

type Processor struct {
	config.Controller
}

func (s *Processor) Run(ctx context.Context, controller *controllercmd.ControllerContext) error {
	klog.Infof("Starting insights-operator processor %s", version.Get().String())
	cont, err := config.LoadConfig(s.Controller, controller.ComponentConfig.Object, config.ToController)
	if err != nil {
		return err
	}
	s.Controller = cont

	// these are operator clients
	// kubeClient, err := kubernetes.NewForConfig(controller.ProtoKubeConfig)
	// if err != nil {
	// 	return err
	// }
	// configClient, err := configv1client.NewForConfig(controller.KubeConfig)
	// if err != nil {
	// 	return err
	// }
	// these are gathering configs
	gatherProtoKubeConfig := rest.CopyConfig(controller.ProtoKubeConfig)
	if len(s.Impersonate) > 0 {
		gatherProtoKubeConfig.Impersonate.UserName = s.Impersonate
	}
	gatherKubeConfig := rest.CopyConfig(controller.KubeConfig)
	if len(s.Impersonate) > 0 {
		gatherKubeConfig.Impersonate.UserName = s.Impersonate
	}

	// the metrics client will connect to prometheus and scrape a small set of metrics
	// TODO: the oauth-proxy and delegating authorizer do not support Impersonate-User,
	//   so we do not impersonate gather
	metricsGatherKubeConfig := rest.CopyConfig(controller.KubeConfig)
	metricsGatherKubeConfig.CAFile = metricCAFile
	metricsGatherKubeConfig.NegotiatedSerializer = scheme.Codecs
	metricsGatherKubeConfig.GroupVersion = &schema.GroupVersion{}
	metricsGatherKubeConfig.APIPath = "/"
	metricsGatherKubeConfig.Host = metricHost

	// If we fail, it's likely due to the service CA not existing yet. Warn and continue,
	// and when the service-ca is loaded we will be restarted.
	// gatherKubeClient, err := kubernetes.NewForConfig(gatherProtoKubeConfig)
	// if err != nil {
	// 	return err
	// }

	// ensure the insight snapshot directory exists
	if _, err = os.Stat(s.StoragePath); err != nil && os.IsNotExist(err) {
		if err = os.MkdirAll(s.StoragePath, 0777); err != nil {
			return fmt.Errorf("can't create --path: %v", err)
		}
	}

	// configobserver synthesizes all config into the status reporter controller
	// configObserver := configobserver.New(s.Controller, kubeClient)
	// go configObserver.Start(ctx)

	// Kafka
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": "192.168.1.34",
		"group.id":          "myGroup",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		panic(err)
	}

	c.SubscribeTopics([]string{"gather", "^aRegex.*[gG]ather"}, nil)

	for {
		msg, err := c.ReadMessage(-1)
		if err == nil {
			klog.V(2).Infof("Message on %s: %s\n", msg.TopicPartition, string(msg.Value))
		} else {
			// The client will automatically try to recover from all errors.
			klog.V(2).Infof("Consumer error: %v (%v)\n", err, msg)
		}
	}

	klog.Warning("stopped")

	<-ctx.Done()
	return nil
}
