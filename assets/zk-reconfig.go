package main

import (
	"flag"
	//	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
)

func main() {
	var (
		addrList    string
		addrListNew string
	)

	flag.StringVar(&addrList, "addrList", "127.0.0.1:2181", "zookeeper address list format as HOST:PORT,HOST:PORT")
	flag.StringVar(&addrListNew, "addrListNew", "127.0.0.1:2181", "zookeeper new address list format as HOST:PORT,HOST:PORT")
	flag.Parse()

	conn, eventChan, err := zk.Connect(strings.Split(addrList, ","), 100*time.Second)
	if err != nil {
		log.Panic("connect to zk server %s: %v", addrList, err)
	}
	defer conn.Close()

	go func() {
		for event := range eventChan {
			log.Printf("event: %+v", event)
		}
	}()

	log.Printf("clusterState: %+v", conn.State().String())
	child, _, _ := conn.Children("/")

	log.Printf("root: %v", child)

	time.Sleep(20 * time.Second)

	log.Printf("clusterState: %+v", conn.State().String())

	newAddrs := strings.Split(addrListNew, ",")
	reconfigAddrs := []string{}
	/*
		for idx, addr := range newAddrs {
			reconfigAddrs = append(reconfigAddrs, fmt.Sprintf("server.%d=%s:2888:3888", idx+1, addr))
		}
	*/
	reconfigAddrs = newAddrs

	log.Printf("reconfigAddrs: %v", reconfigAddrs)

	state, err := conn.Reconfig(reconfigAddrs, -1)
	log.Printf("state: %+v, err: %+v", state, err)

	log.Printf("clusterState: %+v", conn.State().String())

	time.Sleep(60 * time.Second)
}
