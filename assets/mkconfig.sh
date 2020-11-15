#!/bin/sh
podname=${POD_NAME}
namespace=${NAMESPACE}
myid=$((${POD_NAME##*-}+1))
#cp /tmp/zoo.cfg.tpl /conf/zoo.cfg
touch /conf/zoo.cfg
for id in `seq 1 3`
do
  echo -n "server."$id"=zk-"$(($id-1))".zk.zookeeper-cluster:2888:3888;2181\n" >> /conf/zoo.cfg
done
if [ $myid -gt 3 ];then
  echo -n "server."$myid"="$podname".zk.zookeeper-cluster:2888:3888;2181\n" >> /conf/zoo.cfg
fi
