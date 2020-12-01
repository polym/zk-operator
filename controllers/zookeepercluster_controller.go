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

	l.Info("Reconcile")

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

	var (
		nsName  = zkCluster.Namespace
		zkcName = zkCluster.Name

		svcKey = types.NamespacedName{Name: zkcName, Namespace: nsName}
		cmKey  = types.NamespacedName{Name: zkcName, Namespace: nsName}
		stsKey = types.NamespacedName{Name: zkcName, Namespace: nsName}
	)

	// Check service whether exists, create if not found
	svc := new(corev1.Service)
	err = r.Get(ctx, svcKey, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			svcSpec := r.buildService(zkCluster)
			l.Info("Creating a new Service")
			err = r.Create(ctx, svcSpec)
			if err != nil {
				l.Error(err, "Failed to create Service")
				return ctrl.Result{}, err
			}
			l.Info("Create Service ok")
			// l.V(3).Info("Create Service ok")
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Check configmap whether exists, create if not found
	configmap := new(corev1.ConfigMap)
	err = r.Get(ctx, cmKey, configmap)
	if err != nil {
		if errors.IsNotFound(err) {
			configmapSpec := r.buildConfigMap(zkCluster)
			l.Info("Creating a new ConfigMap")
			err = r.Create(ctx, configmapSpec)
			if err != nil {
				l.Error(err, "Failed to create ConfigMap")
				return ctrl.Result{}, err
			}
			l.Info("Create ConfigMap ok")
			// l.V(3).Info("Create ConfigMap ok")
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Check statefulset whether exists, create if not found
	sts := new(appsv1.StatefulSet)
	err = r.Get(ctx, stsKey, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			stsSpec := r.buildStatefulSet(zkCluster)
			l.Info("Creating a new StatefulSet")
			err = r.Create(ctx, stsSpec)
			if err != nil {
				l.Error(err, "Failed to create StatefulSet")
				return ctrl.Result{}, err
			}
			l.Info("Create StatefulSet ok")
			// l.V(3).Info("Create StatefulSet ok")
			// requeue to update status
			return ctrl.Result{Requeue: true}, nil
		}
		l.Error(err, "Failed to get StatefulSet")
		// requeue
		return ctrl.Result{}, err
	}

	// Ensure the statefulset replicas is the same as the spec
	currentReplicas := int(*sts.Spec.Replicas)
	desiredReplicas := zkCluster.Spec.Replicas
	if currentReplicas != desiredReplicas {
		if currentReplicas > desiredReplicas {
			l.Info("reconfigZkScaleDown: %d => %d", currentReplicas, desiredReplicas)
			err = r.reconfigZkScaleDown(nsName, zkcName, currentReplicas, desiredReplicas)
			if err != nil {
				l.Error(err, fmt.Sprintf("Failed to reconfigZkScaleDown %d => %d", currentReplicas, desiredReplicas))
				return ctrl.Result{}, err
			}
		}
		v := int32(desiredReplicas)
		sts.Spec.Replicas = &v
		err = r.Update(ctx, sts)
		if err != nil {
			l.Error(err, "Failed to update StatefulSet")
			return ctrl.Result{}, err
		}
		l.Info("Update StatefulSet ok")
		// l.V(3).Info("Update StatefulSet ok")
		// requeue to update status
		return ctrl.Result{Requeue: true}, nil
	}

	// Sometimes pods in quorum are not desired, scale down it
	qPodNames, err := r.getZkQuorumConfig(nsName, zkcName)
	if err != nil {
		l.Error(err, "Failed to update getZkQuorumConfig")
		return ctrl.Result{}, err
	}
	scaleDown := false
	if len(qPodNames) > int(desiredReplicas) {
		err = r.reconfigZkScaleDown(nsName, zkcName, len(qPodNames), desiredReplicas)
		if err != nil {
			l.Error(err, fmt.Sprintf("Failed to reconfigZkScaleDown %d => %d", len(qPodNames), desiredReplicas))
			return ctrl.Result{}, err
		}
		scaleDown = true
	}

	// Update status and reconfig zookeeper cluster
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(nsName),
		client.MatchingLabels(labelsForZooKeeperCluster(zkcName)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		l.Error(err, "Failed to list pods: %v", listOpts)
		return ctrl.Result{}, err
	}
	podNames, stable := getPodNames(podList.Items)

	if !reflect.DeepEqual(podNames, zkCluster.Status.Nodes) {
		// Only reconfig if not scaleDown case
		if !scaleDown {
			l.Info("reconfigZkWithPods: %v", podList.Items)
			err = r.reconfigZkWithPods(nsName, zkcName, podList.Items)
			if err != nil {
				l.Error(err, "Failed to reconfig zk with pods")
				return ctrl.Result{}, err
			}
		}
		zkCluster.Status.Nodes = podNames
		err = r.Status().Update(ctx, zkCluster)
		if err != nil {
			l.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		l.Info("Update Status ok")
		// requeue to update status
		return ctrl.Result{Requeue: true}, nil
	}

	if !stable || len(podNames) != zkCluster.Spec.Replicas {
		l.Info("Not stable status")
		// l.V(3).Info("Not stable status")
		// requeue until stable
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ZooKeeperClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvv1.ZooKeeperCluster{}).
		Complete(r)
}

func (r *ZooKeeperClusterReconciler) reconfigZkWithPods(namespace, svcName string, pods []corev1.Pod) error {
	addrs := []string{}
	for _, pod := range pods {
		podId, err := strconv.ParseInt(pod.Name[strings.LastIndex(pod.Name, "-")+1:], 0, 0)
		if err != nil {
			return fmt.Errorf("get pod id %s: %v", pod.Name, err)
		}
		addrs = append(addrs, fmt.Sprintf("server.%d=%s.%s.%s.svc.cluster.local:2888:3888", podId+1, pod.Name, svcName, pod.Namespace))
	}

	conn, err := getZkConn(namspace, svcName)
	if err != nil {
		return fmt.Errorf("getZkConn: %v", err)
	}
	defer conn.Close()

	_, err = conn.Reconfig(addrs, -1)
	if err != nil {
		return fmt.Errorf("reconfig failed: %v %v", addrs, err)
	}

	return nil
}

func (r *ZooKeeperClusterReconciler) reconfigZkScaleDown(namespace, svcName string, oldReplicas, newReplicas int) error {
	myids := []string{}
	for i := newReplicas; i < oldReplicas; i++ {
		myids = append(myids, fmt.Sprint(i+1))
	}

	conn, err := getZkConn(namspace, svcName)
	if err != nil {
		return fmt.Errorf("getZkConn: %v", err)
	}
	defer conn.Close()

	_, err = conn.IncrementalReconfig(nil, myids, -1)
	if err != nil {
		return fmt.Errorf("incremental reconfig: %v %v", myids, err)
	}

	return nil
}

func (r *ZooKeeperClusterReconciler) getZkQuorumConfig(namespace, svcName string) ([]string, error) {
	conn, err := getZkConn(namspace, svcName)
	if err != nil {
		return nil, fmt.Errorf("getZkConn: %v", err)
	}
	defer conn.Close()

	data, _, err := conn.Get("/zookeeper/config")
	if err != nil {
		return nil, err
	}
	return getZkQuorumPodNames(string(data)), nil
}

func labelsForZooKeeperCluster(name string) map[string]string {
	return map[string]string{
		"app":          "zookeepercluster",
		"zookeeper_cr": name,
	}
}

func getPodNames(pods []corev1.Pod) ([]string, bool) {
	podNames := make([]string, len(pods))
	stable := true
	for idx, pod := range pods {
		podNames[idx] = fmt.Sprintf("%s/%s", pod.Name, pod.Status.PodIP)
		if pod.Status.PodIP == "" {
			stable = false
		}
	}
	sort.Strings(podNames)
	return podNames, stable
}

/*
server.1=zookeepercluster-sample-0.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
server.2=zookeepercluster-sample-1.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
server.3=zookeepercluster-sample-2.zookeepercluster-sample.default.svc.cluster.local:2888:3888:participant
*/
func getZkQuorumPodNames(data string) []string {
	names := make([]string, 0)
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "server.") {
			names = append(names, strings.Split(strings.Split(line, "=")[1], ".")[0])
		}
	}
	sort.Strings(names)
	return names
}

func getZkConn(namespace, svcName string) (*zk.Conn, error) {
	zkAddrs := []string{}
	// smallest 3 quorum
	for i := 0; i < 3; i++ {
		zkAddrs = append(zkAddrs, fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:2181", svcName, i, svcName, namespace))
	}
	conn, _, err := zk.Connect(zkAddrs, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to zk %s: %v", zkAddrs, err)
	}
	return conn, nil
}
