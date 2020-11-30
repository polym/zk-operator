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
		addrList       string
		addrListAdd    string
		addrListRemove string
	)

	flag.StringVar(&addrList, "addrList", "127.0.0.1:2181", "zookeeper address list format as HOST:PORT,HOST:PORT")
	flag.StringVar(&addrListAdd, "addrListAdd", "127.0.0.1:2888:3888", "zookeeper new address list format as HOST:PORT,HOST:PORT")
	flag.StringVar(&addrListRemove, "addrListRemove", "1", "zookeeper remove myid list")
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

	addAddrs := strings.Split(addrListAdd, ",")
	removeAddrs := strings.Split(addrListRemove, ",")

	log.Printf("add %+v, remove %+v", addAddrs, removeAddrs)

	state, err := conn.IncrementalReconfig(addAddrs, removeAddrs, -1)
	log.Printf("state: %+v, err: %+v", state, err)

	log.Printf("clusterState: %+v", conn.State().String())

	time.Sleep(60 * time.Second)
}
