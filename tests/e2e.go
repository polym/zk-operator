package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	conns    = map[string]*zk.Conn{}
	connLock = sync.Mutex{}
)

func initKubeClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("buildConfigFromFlags: %v", err)
	}
	return kubernetes.NewForConfig(config)
}

func runZkGet(ip string) {
	connLock.Lock()
	defer connLock.Unlock()

	if _, ok := conns[ip]; ok {
		return
	}

	addr := fmt.Sprintf("%s:2181", ip)
	conn, _, err := zk.Connect([]string{addr}, 60*time.Minute)
	if err != nil {
		log.Printf("[E] connect to %s failed", addr)
		return
	}
	log.Printf("[I] new connection to %s", addr)

	conns[ip] = conn

	go func() {
		defer conn.Close()
		for {
			_, _, err := conn.Get("/zookeeper/config")
			if err != nil {
				log.Printf("[E] get %s failed: %v", addr, err)
				connLock.Lock()
				delete(conns, ip)
				connLock.Unlock()
				return
			}
		}
	}()
}

func watchEndpoints(kubeCli kubernetes.Interface, namespace, name string, d time.Duration) {
	ticker := time.NewTicker(d)
	ctx := context.Background()
	for _ = range ticker.C {
		eds, err := kubeCli.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			log.Printf("[E] get endpoint %s/%s: %v", namespace, name, err)
			continue
		}
		for _, ed := range eds.Subsets {
			for _, addr := range ed.Addresses {
				runZkGet(addr.IP)
			}
		}
	}
}

func main() {
	var (
		name       string
		namespace  string
		kubeconfig string
	)
	flag.StringVar(&name, "name", "zk-sample", "zookeepercluster name")
	flag.StringVar(&namespace, "namespace", "default", "zookeepercluster namespace")
	flag.StringVar(&kubeconfig, "kubeconfig", "~/.config/kubeconfig", "kubeconfig path")
	flag.Parse()

	kubeCli, err := initKubeClient(kubeconfig)
	if err != nil {
		log.Fatalf("initKubeClient: %v", err)
	}

	watchEndpoints(kubeCli, namespace, name, time.Second)
}
