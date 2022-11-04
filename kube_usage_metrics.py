import kubernetes.client as k8s_client
import kubernetes.config as k8s_config
from prometheus_client import REGISTRY, make_wsgi_app
from prometheus_client.exposition import ThreadingWSGIServer
from prometheus_client.metrics_core import GaugeMetricFamily
import re
import sys
from wsgiref.simple_server import make_server, WSGIRequestHandler


k8s_config.load_incluster_config()


units = {
    'n': 1.E-9,
    'u': 1.E-6,
    'm': 1.E-3,
    '': 1,
    'k': 1.E+3,
    'M': 1.E+6,
    'G': 1.E+9,
    'T': 1.E+12,
    'P': 1.E+15,
    'E': 1.E+18,

    'Ki': 2. ** 10,
    'Mi': 2. ** 20,
    'Gi': 2. ** 30,
    'Ti': 2. ** 40,
    'Pi': 2. ** 50,
    'Ei': 2. ** 60,
}


def parse_cpu(val):
    number, unit = re.match(r'^([0-9.]+)([a-zA-Z]*)$', val).groups()
    number = float(number)
    try:
        return number * units[unit]
    except KeyError:
        print("Warning: unknown cpu unit %r" % unit, file=sys.stderr)
        return 0


def parse_memory(val):
    number, unit = re.match(r'^([0-9.]+)([a-zA-Z]*)$', val).groups()
    number = float(number)
    try:
        return number * units[unit]
    except KeyError:
        print("Warning: unknown memory unit %r" % unit, file=sys.stderr)
        return 0


class Collector(object):
    def collect(self):
        namespaces = {}

        with k8s_client.ApiClient() as api:
            res, status, headers = api.call_api(
                '/apis/metrics.k8s.io/v1beta1/pods', 'GET',
                response_type=object,
                auth_settings=['BearerToken'],
            )
            if status != 200:
                raise ValueError("Error %d" % status)

            for pod in res['items']:
                try:
                    ns = namespaces[pod['metadata']['namespace']]
                except KeyError:
                    ns = {'cpu': 0, 'memory': 0}
                    namespaces[pod['metadata']['namespace']] = ns
                for container in pod['containers']:
                    ns['cpu'] += parse_cpu(container['usage']['cpu'])
                    ns['memory'] += parse_memory(container['usage']['memory'])

        cpu = GaugeMetricFamily(
            'namespace_cpu',
            "CPU usage per namespace",
            labels=['namespace'],
        )
        memory = GaugeMetricFamily(
            'namespace_memory_bytes',
            "Memory usage per namespace",
            labels=['namespace'],
        )
        for ns, resources in namespaces.items():
            cpu.add_metric([ns], resources['cpu'])
            memory.add_metric([ns], resources['memory'])

        return [cpu, memory]


REGISTRY.register(Collector())


class SilentHandler(WSGIRequestHandler):
    """WSGI handler that does not log requests."""

    def log_message(self, format, *args):
        """Log nothing."""


httpd = make_server(
    '0.0.0.0', 8080,
    make_wsgi_app(),
    ThreadingWSGIServer, handler_class=SilentHandler,
)
httpd.serve_forever()
