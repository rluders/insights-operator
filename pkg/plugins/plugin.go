package plugins

import (
	"context"

	"github.com/openshift/insights-operator/pkg/record"
	"k8s.io/client-go/rest"
)

// Plugin defines the gather plugin interface
type Plugin interface {
	Gather(context.Context, *rest.Config) func() ([]record.Record, []error)
}
