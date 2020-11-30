/*
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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	kvv1 "github.com/polym/zk-operator/api/v1"
)

var configMapData = map[string]string{
	"zoo.cfg": `dataDir=/data
dataLogDir=/datalog
tickTime=2000
initLimit=10
syncLimit=2
autopurge.snapRetainCount=3
autopurge.purgeInterval=0
maxClientCnxns=60
standaloneEnabled=false
reconfigEnabled=true
skipACL=yes
admin.enableServer=true
4lw.commands.whitelist=*
`,
	"mkconfig.sh": `#!/bin/sh
podname=${POD_NAME}
namespace=${NAMESPACE}
myid=$((${POD_NAME##*-}+1))
svcname=${POD_NAME%-*}
echo $myid > /data/myid
cp /tmp/zoo.cfg.tpl /conf/zoo.cfg
for id in $(seq 1 3)
do
  echo -e "server."$id"=$svcname-"$(($id-1))".$svcname.$namespace.svc.cluster.local:2888:3888;2181" >> /conf/zoo.cfg
done
if [ $myid -gt 3 ];then
  echo -e "server."$myid"="$podname".$svcname.$namespace.svc.cluster.local:2888:3888;2181" >> /conf/zoo.cfg
fi
`,
}

// TODO(@hongbo.mo): load from global configmap
func (r *ZooKeeperClusterReconciler) buildConfigMap(spec *kvv1.ZooKeeperCluster) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Data: configMapData,
	}
	ctrl.SetControllerReference(spec, cm, r.Scheme)
	return cm
}

func (r *ZooKeeperClusterReconciler) buildService(spec *kvv1.ZooKeeperCluster) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "port-1", Port: 2181},
			},
			Selector: labelsForZooKeeperCluster(spec.Name),
		},
	}
	ctrl.SetControllerReference(spec, svc, r.Scheme)
	return svc
}

func (r *ZooKeeperClusterReconciler) buildStatefulSet(spec *kvv1.ZooKeeperCluster) *appsv1.StatefulSet {
	replicas := int32(spec.Spec.Replicas)
	labels := labelsForZooKeeperCluster(spec.Name)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: spec.Name,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "zk",
							Image:           "zookeeper:3.5.8",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{Name: "port1", ContainerPort: 2181},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config-volume", SubPath: "zoo.cfg", MountPath: "/conf/zoo.cfg"},
								{Name: "data-volume", SubPath: "myid", MountPath: "/data/myid"},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            "cfg-maker",
							Image:           "busybox",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh"},
							Args:            []string{"/mkconfig.sh"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "zk-cfg", SubPath: "zoo.cfg", MountPath: "/tmp/zoo.cfg.tpl"},
								{Name: "zk-cfg", SubPath: "mkconfig.sh", MountPath: "/mkconfig.sh"},
								{Name: "config-volume", MountPath: "/conf"},
								{Name: "data-volume", MountPath: "/data"},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "zk-cfg",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: spec.Name},
								},
							},
						},
						{
							Name: "config-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "data-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(spec, sts, r.Scheme)
	return sts
}
