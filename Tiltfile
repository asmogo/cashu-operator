CLUSTER_NAME = 'cashu-operator-dev'
CLUSTER_CONTEXT = 'k3d-' + CLUSTER_NAME
CERT_MANAGER_VERSION = 'v1.20.2'
INGRESS_NGINX_VERSION = 'controller-v1.15.1'

allow_k8s_contexts(CLUSTER_CONTEXT)

if k8s_context() != CLUSTER_CONTEXT:
    fail('Use `make tilt-up` or switch kubectl to %s before starting Tilt.' % CLUSTER_CONTEXT)

# The operator watches cert-manager Certificate resources via Owns(), so the
# CRDs must exist before the controller-manager starts. Install cert-manager
# once and wait for the webhook to be ready.
local_resource(
    'cert-manager',
    cmd=' && '.join([
        'kubectl --context %s apply -f https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml' % (CLUSTER_CONTEXT, CERT_MANAGER_VERSION),
        'kubectl --context %s -n cert-manager wait --for=condition=Available --timeout=180s deploy --all' % CLUSTER_CONTEXT,
    ]),
    labels=['infra'],
    allow_parallel=True,
)

# ingress-nginx is the default ingress class the operator generates for mint
# ingresses. k3d's loadbalancer exposes host ports 80/443 (see ctlptl-config),
# so hitting http://<host>.localhost reaches the controller, which routes to
# the mint service.
local_resource(
    'ingress-nginx',
    cmd=' && '.join([
        'kubectl --context %s apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/%s/deploy/static/provider/cloud/deploy.yaml' % (CLUSTER_CONTEXT, INGRESS_NGINX_VERSION),
        'kubectl --context %s -n ingress-nginx wait --for=condition=Available --timeout=180s deploy/ingress-nginx-controller' % CLUSTER_CONTEXT,
    ]),
    labels=['infra'],
    allow_parallel=True,
)

docker_build(
    'controller:latest',
    '.',
    dockerfile='Dockerfile',
    only=['api', 'cmd', 'internal', 'go.mod', 'go.sum', 'Dockerfile'],
)

k8s_yaml(kustomize('config/default'))

local_resource(
    'codegen',
    cmd='make manifests generate',
    deps=['api', 'hack/boilerplate.go.txt'],
    ignore=['api/**/zz_generated.deepcopy.go'],
)

local_resource(
    'unit-tests',
    cmd='make test',
    deps=['api', 'cmd', 'internal', 'go.mod', 'go.sum'],
    resource_deps=['codegen'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
)

local_resource(
    'demo-mint',
    cmd='kubectl --context %s apply -k config/dev' % CLUSTER_CONTEXT,
    deps=['config/dev'],
    resource_deps=['manager', 'ingress-nginx'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
)

local_resource(
    'demo-mint-delete',
    cmd='kubectl --context %s delete -k config/dev --ignore-not-found' % CLUSTER_CONTEXT,
    deps=['config/dev'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
)

local_resource(
    'demo-orchard',
    cmd='kubectl --context %s apply -k config/dev/orchard' % CLUSTER_CONTEXT,
    deps=['config/dev/orchard'],
    resource_deps=['manager', 'ingress-nginx'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
)

local_resource(
    'demo-orchard-delete',
    cmd='kubectl --context %s delete -k config/dev/orchard --ignore-not-found' % CLUSTER_CONTEXT,
    deps=['config/dev/orchard'],
    trigger_mode=TRIGGER_MODE_MANUAL,
    auto_init=False,
)

k8s_resource(
    'cashu-operator-controller-manager',
    new_name='manager',
    labels=['operator'],
    resource_deps=['codegen', 'cert-manager'],
)
