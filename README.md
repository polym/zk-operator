
[![Go Report Card](https://goreportcard.com/badge/github.com/polym/zk-operator)](https://goreportcard.com/report/github.com/polym/zk-operator)

## Overview

ZooKeeperCluster Operator is developed by [operator-sdk](https://github.com/operator-framework/operator-sdk) toolkits. Now it supports to scale up/down zookeeper cluster nodes.

## Build

```
$ make docker-build IMG=your-image-name
```

## Deploy

```
$ make deploy IMG=your-image-name
```

## YAML

```
apiVersion: kv.polym.xyz/v1
kind: ZooKeeperCluster
metadata:
  name: zk-sample
spec:
  replicas: 5
```

## Addition: Kubernetes dev environment init

```
$ kubeadm init --pod-network-cidr=10.244.0.0/16
$ kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
```
