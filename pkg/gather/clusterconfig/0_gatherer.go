package clusterconfig

import (
	"context"
	"fmt"
	"plugin"
	"sync"

	"k8s.io/client-go/rest"
	"k8s.io/klog"

	configv1 "github.com/openshift/api/config/v1"
	_ "k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"github.com/openshift/insights-operator/pkg/plugins"
	"github.com/openshift/insights-operator/pkg/record"
)

// Gatherer is a driving instance invoking collection of data
type Gatherer struct {
	ctx                     context.Context
	gatherKubeConfig        *rest.Config
	gatherProtoKubeConfig   *rest.Config
	metricsGatherKubeConfig *rest.Config
	lock                    sync.Mutex
	lastVersion             *configv1.ClusterVersion
}

// New creates new Gatherer
func New(gatherKubeConfig *rest.Config, gatherProtoKubeConfig *rest.Config, metricsGatherKubeConfig *rest.Config) *Gatherer {
	return &Gatherer{
		gatherKubeConfig:        gatherKubeConfig,
		gatherProtoKubeConfig:   gatherProtoKubeConfig,
		metricsGatherKubeConfig: metricsGatherKubeConfig,
	}
}

// Gather is hosting and calling all the recording functions
func (g *Gatherer) Gather(ctx context.Context, recorder record.Interface) error {
	g.ctx = ctx

	// @FIXME It can be loaded by configuration
	gathersPlugins := []string{"container_runtime_configs.so", "pod_disruption_budgets.so"}
	gathers := make([]func() ([]record.Record, []error), 0)

	for _, v := range gathersPlugins {
		p, err := loadPlugin(v)

		if err != nil {
			// @TODO Log stuff
			klog.V(2).Infof("Gatherer %v not found", v)
			continue
		}
		klog.V(2).Infof("Loadding gather %s", v)
		gathers = append(gathers, p.Gather(ctx, g.gatherKubeConfig))
	}

	return record.Collect(ctx, recorder, gathers)
	// GatherPodDisruptionBudgets(g),
	// GatherMostRecentMetrics(g),
	// GatherClusterOperators(g),
	// GatherContainerImages(g),
	// GatherNodes(g),
	// GatherConfigMaps(g),
	// GatherClusterVersion(g),
	// GatherClusterID(g),
	// GatherClusterInfrastructure(g),
	// GatherClusterNetwork(g),
	// GatherClusterAuthentication(g),
	// GatherClusterImageRegistry(g),
	// GatherClusterImagePruner(g),
	// GatherClusterFeatureGates(g),
	// GatherClusterOAuth(g),
	// GatherClusterIngress(g),
	// GatherClusterProxy(g),
	// GatherCertificateSigningRequests(g),
	// GatherCRD(g),
	// GatherHostSubnet(g),
	// GatherMachineSet(g),
	// GatherInstallPlans(g),
	// GatherServiceAccounts(g),
	// GatherMachineConfigPool(g),
	// GatherContainerRuntimeConfig(g),
	// GatherStatefulSets(g)

}

func (g *Gatherer) setClusterVersion(version *configv1.ClusterVersion) {
	g.lock.Lock()
	defer g.lock.Unlock()
	if g.lastVersion != nil && g.lastVersion.ResourceVersion == version.ResourceVersion {
		return
	}
	g.lastVersion = version.DeepCopy()
}

// ClusterVersion returns Version for this cluster, which is set by running version during Gathering
func (g *Gatherer) ClusterVersion() *configv1.ClusterVersion {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.lastVersion
}

func loadPlugin(path string) (plugins.Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	v, err := p.Lookup("Plugin")
	if err != nil {
		return nil, err
	}

	o, ok := v.(plugins.Plugin)
	if !ok {
		return nil, fmt.Errorf("Unable to cast plugin")
	}

	return o, nil
}
