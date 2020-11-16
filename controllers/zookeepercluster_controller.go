/*
Copyright 2020.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-zookeeper/zk"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kvv1 "github.com/polym/zk-operator/api/v1"
)

// ZooKeeperClusterReconciler reconciles a ZooKeeperCluster object
type ZooKeeperClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kv.polym.xyz,resources=zookeeperclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kv.polym.xyz,resources=zookeeperclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;

func (r *ZooKeeperClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	l := r.Log.WithValues("zookeepercluster", req.NamespacedName)

	zkCluster := new(kvv1.ZooKeeperCluster)
	err := r.Get(ctx, req.NamespacedName, zkCluster)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("ZooKeeperCluster not found. Ignore since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ZooKeeperCluster")
		return ctrl.Result{}, err
	}

	// name for statefulset, configmap, service
	namespacedName := types.NamespacedName{Name: zkCluster.Name, Namespace: zkCluster.Namespace}

	// Check service whether exists, create if not found
	svc := new(corev1.Service)
	err = r.Get(ctx, namespacedName, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			svcSpec := r.buildService(zkCluster)
			l.Info("Creating a new Service")
			err = r.Create(ctx, svcSpec)
			if err != nil {
				l.Error(err, "Failed to create Service")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Check configmap whether exists, create if not found
	configmap := new(corev1.ConfigMap)
	err = r.Get(ctx, namespacedName, configmap)
	if err != nil {
		if errors.IsNotFound(err) {
			configmapSpec := r.buildConfigMap(zkCluster)
			l.Info("Creating a new ConfigMap")
			err = r.Create(ctx, configmapSpec)
			if err != nil {
				l.Error(err, "Failed to create ConfigMap")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Check statefulset whether exists, create if not found
	sts := new(appsv1.StatefulSet)
	err = r.Get(ctx, namespacedName, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			stsSpec := r.buildStatefulSet(zkCluster)
			l.Info("Creating a new StatefulSet")
			err = r.Create(ctx, stsSpec)
			if err != nil {
				l.Error(err, "Failed to create StatefulSet")
				return ctrl.Result{}, err
			}
			// requeue to update status
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get StatefulSet")
		// requeue
		return ctrl.Result{}, err
	}

	// Ensure the statefulset replicas is the same as the spec
	desiredReplicas := int32(zkCluster.Spec.Replicas)
	if *sts.Spec.Replicas != desiredReplicas {
		sts.Spec.Replicas = &desiredReplicas
		err = r.Update(ctx, sts)
		if err != nil {
			l.Error(err, "Failed to update StatefulSet")
			return ctrl.Result{}, err
		}
		// requeue to update status
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status and reconfig zookeeper cluster
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(zkCluster.Namespace),
		client.MatchingLabels(labelsForZooKeeperCluster(zkCluster.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		l.Error(err, "Failed to list pods: %v", listOpts)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	if !reflect.DeepEqual(podNames, zkCluster.Status.Nodes) {
		err = r.reconfigZk(zkCluster.Namespace, zkCluster.Name, podList.Items)
		if err != nil {
			l.Error(err, "Failed to reconfig zk")
			return ctrl.Result{}, err
		}
		zkCluster.Status.Nodes = podNames
		err = r.Status().Update(ctx, zkCluster)
		if err != nil {
			l.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ZooKeeperClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvv1.ZooKeeperCluster{}).
		Complete(r)
}

func (r *ZooKeeperClusterReconciler) reconfigZk(namespace, svcName string, pods []corev1.Pod) error {
	addrs := []string{}
	for _, pod := range pods {
		podId, err := strconv.ParseInt(pod.Name[strings.LastIndex(pod.Name, "-")+1:], 0, 0)
		if err != nil {
			return fmt.Errorf("get pod id %s: %v", pod.Name, err)
		}
		addrs = append(addrs, fmt.Sprintf("server.%d=%s.%s.%s.svc.cluster.local:2888:3888", podId+1, pod.Name, svcName, pod.Namespace))
	}

	zkAddr := fmt.Sprintf("%s.%s.svc.cluster.local:2181", svcName, namespace)
	conn, _, err := zk.Connect([]string{zkAddr}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect to zk %s: %v", zkAddr, err)
	}
	defer conn.Close()

	_, err = conn.Reconfig(addrs, -1)
	if err != nil {
		return fmt.Errorf("reconfig failed: %v %v", addrs, err)
	}

	return nil
}

func labelsForZooKeeperCluster(name string) map[string]string {
	return map[string]string{
		"app":          "zookeepercluster",
		"zookeeper_cr": name,
	}
}

func getPodNames(pods []corev1.Pod) []string {
	podNames := make([]string, len(pods))
	for idx, pod := range pods {
		podNames[idx] = fmt.Sprintf("%s/%s", pod.Name, pod.Status.PodIP)
	}
	sort.Strings(podNames)
	return podNames
}
