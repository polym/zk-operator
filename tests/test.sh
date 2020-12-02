while true;
do
    r=$(echo $RANDOM % 20 + 1 | bc)
    kubectl patch zkc zookeepercluster-sample -p '{"spec":{"replicas":'${r}'}}' --type=merge
    echo "scale" $r
    sleep 30
done
