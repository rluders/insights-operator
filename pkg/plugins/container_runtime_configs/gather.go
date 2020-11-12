package main

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	_ "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/openshift/insights-operator/pkg/record"
)

type plugin struct{}

// GatherContainerRuntimeConfig collects ContainerRuntimeConfig  information
//
// The Kubernetes api https://github.com/openshift/machine-config-operator/blob/master/pkg/apis/machineconfiguration.openshift.io/v1/types.go#L402
// Response see https://docs.okd.io/latest/rest_api/machine_apis/containerruntimeconfig-machineconfiguration-openshift-io-v1.html
//
// Location in archive: config/containerruntimeconfigs/
func (p *plugin) Gather(ctx context.Context, kubeConfig *rest.Config) func() ([]record.Record, []error) {
	return func() ([]record.Record, []error) {
		dynamicClient, err := dynamic.NewForConfig(kubeConfig)
		if err != nil {
			return nil, []error{err}
		}
		return p.containerRuntimeConfig(ctx, dynamicClient)
	}
}

func (p *plugin) containerRuntimeConfig(ctx context.Context, dynamicClient dynamic.Interface) ([]record.Record, []error) {
	crc := schema.GroupVersionResource{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "containerruntimeconfigs"}
	containerRCs, err := dynamicClient.Resource(crc).List(ctx, metav1.ListOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}

	records := []record.Record{}
	for _, i := range containerRCs.Items {
		records = append(records, record.Record{
			Name: fmt.Sprintf("config/containerruntimeconfigs/%s", i.GetName()),
			Item: record.JSONMarshaller{Object: i.Object},
		})
	}
	return records, nil
}

var Plugin plugin
