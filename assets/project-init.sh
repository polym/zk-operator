operator-sdk init --domain=polym.xyz --repo=github.com/polym/zk-operator
operator-sdk create api --group kv --version v1 --kind ZooKeeperCluster --resource=true --controller=true
