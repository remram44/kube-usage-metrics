package main

import (
    "context"

    "k8s.io/client-go/tools/clientcmd"
    clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
    "k8s.io/client-go/rest"
    "k8s.io/apimachinery/pkg/api/resource"

    "log"
    "net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var Logger = log.Default()

type Metrics struct {
    Cpu *resource.Quantity
    Memory *resource.Quantity
}

func GetNamespaceMetrics(ctx context.Context, clientset *metricsv.Clientset) (map[string]Metrics, error) {
    namespaces := make(map[string]Metrics)

    podMetricsList, err := clientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }

    // Iterate on pods
    for _, pod := range podMetricsList.Items {
        ns, ok := namespaces[pod.Namespace]
        if !ok {
            ns = Metrics{Cpu: resource.NewQuantity(0, resource.DecimalSI), Memory: resource.NewQuantity(0, resource.DecimalSI)}
            namespaces[pod.Namespace] = ns
        }

        // Iterate on containers
        for _, container := range pod.Containers {
            ns.Cpu.Add(container.Usage["cpu"])
            ns.Memory.Add(container.Usage["memory"])
        }
    }

    return namespaces, nil
}

type Collector struct {
    prometheus.Collector
    Clientset *metricsv.Clientset
}

var (
    namespaceCpuDesc = prometheus.NewDesc(
        "namespace_cpu",
        "CPU usage per namespace",
        []string{"namespace"}, nil,
    )
    namespaceMemoryDesc = prometheus.NewDesc(
        "namespace_memory_bytes",
        "Memory usage per namespace",
        []string{"namespace"}, nil,
    )
)

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
    prometheus.DescribeByCollect(c, ch)
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    namespaces, err := GetNamespaceMetrics(ctx, c.Clientset)
    if err != nil {
        Logger.Print(err.Error())
    }

    for name, ns := range namespaces {
        ch <- prometheus.MustNewConstMetric(
            namespaceCpuDesc,
            prometheus.GaugeValue,
            ns.Cpu.AsApproximateFloat64(),
            name,
        )
        ch <- prometheus.MustNewConstMetric(
            namespaceMemoryDesc,
            prometheus.GaugeValue,
            ns.Memory.AsApproximateFloat64(),
            name,
        )
    }
}

func main() {
    // Load in-cluster config
    //config, err := rest.InClusterConfig()
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        &clientcmd.ClientConfigLoadingRules{ExplicitPath: "/home/remram/.kube/configs/nyuhsrn"},
        &clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
    if err != nil {
        Logger.Fatal("Can't load config", err.Error())
    }

    config.UserAgent = "kube-usage-metrics"
    httpClient, err := rest.HTTPClientFor(config)

    clientset, err := metricsv.NewForConfigAndClient(config, httpClient)

    prometheus.MustRegister(Collector{Clientset: clientset})
    http.Handle("/metrics", promhttp.Handler())
    log.Fatal(http.ListenAndServe(":8080", nil))
}
