/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prometheusoperator

import (
	"context"
	"testing"

	api "kmodules.xyz/monitoring-agent-api/api/v1"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promfake "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// fakeDiscovery reports the ServiceMonitor kind; the stock fake returns no preferred resources.
type fakeDiscovery struct {
	*fakediscovery.FakeDiscovery
}

func (d fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return []*metav1.APIResourceList{{
		GroupVersion: promapi.SchemeGroupVersion.String(),
		APIResources: []metav1.APIResource{{Name: "servicemonitors", Kind: promapi.ServiceMonitorsKind}},
	}}, nil
}

type fakeKube struct {
	*kubefake.Clientset
}

func (c fakeKube) Discovery() discovery.DiscoveryInterface {
	return fakeDiscovery{c.Clientset.Discovery().(*fakediscovery.FakeDiscovery)}
}

var _ kubernetes.Interface = fakeKube{}

type stats struct{}

func (stats) GetNamespace() string                              { return "demo" }
func (stats) ServiceName() string                               { return "db-stats" }
func (stats) ServiceMonitorName() string                        { return "db-stats" }
func (stats) ServiceMonitorAdditionalLabels() map[string]string { return nil }
func (stats) Path() string                                      { return "/metrics" }
func (stats) Scheme() string                                    { return "http" }
func (stats) TLSConfig() *promapi.TLSConfig                     { return nil }

// The pod label must come from a meta label that both the Endpoints and the
// EndpointSlice discovery roles set; __meta_kubernetes_endpoint_address_target_name
// is empty under EndpointSlice and a replace from it removes the pod label.
func TestCreateOrUpdatePodRelabel(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "db-stats", Namespace: "demo", Labels: map[string]string{"app": "db"}},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "metrics", Port: 56790}}},
	}
	kc := fakeKube{kubefake.NewSimpleClientset(svc)}
	pc := promfake.NewSimpleClientset().MonitoringV1()

	spec := &api.AgentSpec{
		Agent: api.AgentPrometheusOperator,
		Prometheus: &api.PrometheusSpec{ServiceMonitor: &api.ServiceMonitorSpec{
			Interval: "10s",
			Endpoints: []api.Endpoint{{
				Port: "metrics",
				RelabelConfigs: []promapi.RelabelConfig{{
					SourceLabels: []promapi.LabelName{"__meta_kubernetes_pod_label_role"},
					TargetLabel:  "role",
				}},
			}},
		}},
	}
	if _, err := New(api.AgentPrometheusOperator, kc, pc).CreateOrUpdate(stats{}, spec); err != nil {
		t.Fatalf("CreateOrUpdate: %v", err)
	}

	sm, err := pc.ServiceMonitors("demo").Get(context.TODO(), "db-stats", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get ServiceMonitor: %v", err)
	}
	if len(sm.Spec.Endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(sm.Spec.Endpoints))
	}
	rcs := sm.Spec.Endpoints[0].RelabelConfigs
	if len(rcs) != 2 {
		t.Fatalf("want the pod relabel followed by the user relabel, got %+v", rcs)
	}

	pod := rcs[0]
	if len(pod.SourceLabels) != 1 || pod.SourceLabels[0] != "__meta_kubernetes_pod_name" {
		t.Errorf("pod relabel source = %v, want [__meta_kubernetes_pod_name]", pod.SourceLabels)
	}
	if pod.TargetLabel != "pod" || pod.Action != "replace" {
		t.Errorf("pod relabel = %+v, want replace into pod", pod)
	}
	if pod.Regex != "(.+)" {
		t.Errorf("pod relabel regex = %q, want (.+) so an empty source keeps pod", pod.Regex)
	}
	if rcs[1].TargetLabel != "role" {
		t.Errorf("user relabel not kept after the pod relabel: %+v", rcs[1])
	}
}
