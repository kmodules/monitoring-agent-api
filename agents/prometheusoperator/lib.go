/*
Copyright The Kmodules Authors.

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

	kutil "kmodules.xyz/client-go"
	core_util "kmodules.xyz/client-go/core/v1"
	api "kmodules.xyz/monitoring-agent-api/api/v1"

	promapi "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	prom "github.com/coreos/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

var json = jsoniter.ConfigFastest

// PrometheusCoreosOperator creates `ServiceMonitor` so that CoreOS Prometheus operator can generate necessary config for Prometheus.
type PrometheusCoreosOperator struct {
	at         api.AgentType
	k8sClient  kubernetes.Interface
	promClient prom.MonitoringV1Interface
}

func New(at api.AgentType, k8sClient kubernetes.Interface, promClient prom.MonitoringV1Interface) api.Agent {
	return &PrometheusCoreosOperator{
		at:         at,
		k8sClient:  k8sClient,
		promClient: promClient,
	}
}

func (agent *PrometheusCoreosOperator) GetType() api.AgentType {
	return agent.at
}

func CreateOrPatchServiceMonitor(ctx context.Context, c prom.MonitoringV1Interface, meta metav1.ObjectMeta, transform func(monitor *promapi.ServiceMonitor) *promapi.ServiceMonitor, opts metav1.PatchOptions) (*promapi.ServiceMonitor, kutil.VerbType, error) {
	cur, err := c.ServiceMonitors(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating ServiceMonitor %s/%s.", meta.Namespace, meta.Name)
		out, err := c.ServiceMonitors(meta.Namespace).Create(ctx, transform(&promapi.ServiceMonitor{
			TypeMeta: metav1.TypeMeta{
				Kind:       promapi.PrometheusesKind,
				APIVersion: promapi.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}), metav1.CreateOptions{
			DryRun:       opts.DryRun,
			FieldManager: opts.FieldManager,
		})
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchServiceMonitor(ctx, c, cur, transform, opts)
}

func PatchServiceMonitor(ctx context.Context, c prom.MonitoringV1Interface, cur *promapi.ServiceMonitor, transform func(monitor *promapi.ServiceMonitor) *promapi.ServiceMonitor, opts metav1.PatchOptions) (*promapi.ServiceMonitor, kutil.VerbType, error) {
	return PatchServiceMonitorbject(ctx, c, cur, transform(cur.DeepCopy()), opts)
}

func PatchServiceMonitorbject(ctx context.Context, c prom.MonitoringV1Interface, cur, mod *promapi.ServiceMonitor, opts metav1.PatchOptions) (*promapi.ServiceMonitor, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := jsonpatch.CreateMergePatch(curJson, modJson)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching ServiceMonitor %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.ServiceMonitors(cur.Namespace).Patch(ctx, cur.Name, types.MergePatchType, patch, opts)
	return out, kutil.VerbPatched, err
}

func TryUpdateStatefulSet(ctx context.Context, c prom.MonitoringV1Interface, meta metav1.ObjectMeta, transform func(monitor *promapi.ServiceMonitor) *promapi.ServiceMonitor, opts metav1.UpdateOptions) (result *promapi.ServiceMonitor, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.ServiceMonitors(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.ServiceMonitors(cur.Namespace).Update(ctx, transform(cur.DeepCopy()), opts)
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update ServiceMonitor %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update ServiceMonitor %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}

func (agent *PrometheusCoreosOperator) CreateOrUpdate(sp api.StatsAccessor, new *api.AgentSpec) (kutil.VerbType, error) {
	if !agent.isOperatorInstalled() {
		return kutil.VerbUnchanged, errors.New("cluster does not support CoreOS Prometheus operator")
	}

	svc, err := agent.k8sClient.CoreV1().Services(sp.GetNamespace()).Get(context.TODO(), sp.ServiceName(), metav1.GetOptions{})
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	var portName string
	for _, p := range svc.Spec.Ports {
		if p.Port == new.Prometheus.Exporter.Port {
			portName = p.Name
		}
	}
	if portName == "" {
		return kutil.VerbUnchanged, errors.New("no port found in stats service")
	}

	smMeta := metav1.ObjectMeta{
		Name:      sp.ServiceMonitorName(),
		Namespace: sp.GetNamespace(),
	}
	owner := metav1.NewControllerRef(svc, corev1.SchemeGroupVersion.WithKind("Service"))

	_, vt, err := CreateOrPatchServiceMonitor(context.TODO(), agent.promClient, smMeta, func(in *promapi.ServiceMonitor) *promapi.ServiceMonitor {
		in.Labels = new.Prometheus.ServiceMonitor.Labels
		in.Spec.NamespaceSelector = promapi.NamespaceSelector{
			MatchNames: []string{sp.GetNamespace()},
		}
		in.Spec.Endpoints = []promapi.Endpoint{
			{
				Port:        portName,
				Interval:    new.Prometheus.ServiceMonitor.Interval,
				Path:        sp.Path(),
				HonorLabels: true,
			},
		}
		in.Spec.Selector = metav1.LabelSelector{
			MatchLabels: svc.Labels,
		}
		core_util.EnsureOwnerReference(&in.ObjectMeta, owner)
		return in
	}, metav1.PatchOptions{})

	return vt, nil
}

func (agent *PrometheusCoreosOperator) Delete(sp api.StatsAccessor) (kutil.VerbType, error) {
	if !agent.isOperatorInstalled() {
		return kutil.VerbUnchanged, errors.New("cluster does not support CoreOS Prometheus operator")
	}

	err := agent.promClient.ServiceMonitors(sp.GetNamespace()).Delete(context.TODO(), sp.ServiceMonitorName(), metav1.DeleteOptions{})
	if err != nil {
		return kutil.VerbUnchanged, err
	}
	return kutil.VerbDeleted, nil
}

func (agent *PrometheusCoreosOperator) isOperatorInstalled() bool {
	if resourceList, err := agent.k8sClient.Discovery().ServerPreferredResources(); discovery.IsGroupDiscoveryFailedError(err) || err == nil {
		for _, resources := range resourceList {
			gv, err := schema.ParseGroupVersion(resources.GroupVersion)
			if err != nil {
				return false
			}
			if gv.Group != promapi.SchemeGroupVersion.Group {
				continue
			}
			for _, resource := range resources.APIResources {
				if resource.Kind == promapi.PrometheusesKind || resource.Kind == promapi.ServiceMonitorsKind {
					return true
				}
			}
		}
	}
	return false
}
