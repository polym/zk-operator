docker run -d --name=zk-01 -e ZOO_MY_ID=1 -v $(pwd)/zoo-01.cfg:/conf/zoo.cfg zookeeper:3.5.8
docker run -d --name=zk-02 -e ZOO_MY_ID=2 -v $(pwd)/zoo-02.cfg:/conf/zoo.cfg zookeeper:3.5.8
docker run -d --name=zk-03 -e ZOO_MY_ID=3 -v $(pwd)/zoo-03.cfg:/conf/zoo.cfg zookeeper:3.5.8
