package main

import (
    "context"
    "fmt"
    "k8s.io/client-go/tools/clientcmd"
    clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
    rest "k8s.io/client-go/rest"
    "k8s.io/apimachinery/pkg/api/resource"
)

type Metrics struct {
    Cpu *resource.Quantity
    Memory *resource.Quantity
}

func main() {
    // Load in-cluster config
    //config, err := rest.InClusterConfig()
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        &clientcmd.ClientConfigLoadingRules{ExplicitPath: "/home/remram/.kube/configs/nyuhsrn"},
        &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
    if err != nil {
        panic(err.Error())
    }

    config.UserAgent = "kube-usage-metrics"
    httpClient, err := rest.HTTPClientFor(config)

    clientset, err := metricsv.NewForConfigAndClient(config, httpClient)
    podMetricsList, err := clientset.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
    namespaces := make(map[string]Metrics)
    for _, pod := range podMetricsList.Items {
        ns, ok := namespaces[pod.Namespace]
        if !ok {
            ns = Metrics{Cpu: resource.NewQuantity(0, resource.DecimalSI), Memory: resource.NewQuantity(0, resource.DecimalSI)}
            namespaces[pod.Namespace] = ns
        }
        for _, container := range pod.Containers {
            ns.Cpu.Add(container.Usage["cpu"])
            ns.Memory.Add(container.Usage["memory"])
        }
    }

    for name, ns := range namespaces {
        fmt.Printf("ns: %v\n    CPU: %v\n    Memory: %v\n", name, ns.Cpu, ns.Memory)
    }
}
