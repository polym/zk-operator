namespace=$1
name=$2
for ip in $(kubectl -n $namespace get pod -o wide | grep $name | awk '{print $6}')
do
  echo -n "$ip "
  echo srvr | nc $ip 2181  | grep Mode
  echo srvr | nc $ip 2181  | grep "Node count"
done
