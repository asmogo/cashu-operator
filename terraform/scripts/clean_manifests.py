#!/usr/bin/env python3
"""Clean a `kubectl get -o json` List into tidy multi-doc YAML for Terraform's
kubectl provider. Strips cluster-managed/server-side fields so the manifests
re-apply cleanly on a fresh cluster."""
import sys, json, yaml

DROP_META = {
    "uid", "resourceVersion", "generation", "creationTimestamp", "selfLink",
    "managedFields", "ownerReferences", "finalizers",
}
DROP_ANNOS = {
    "kubectl.kubernetes.io/last-applied-configuration",
    "deployment.kubernetes.io/revision",
    "autoscaling.alpha.kubernetes.io/conditions",
    "cloud.google.com/neg-status",
    "ingress.kubernetes.io/backends",
    "ingress.kubernetes.io/forwarding-rule",
    "ingress.kubernetes.io/target-proxy",
    "ingress.kubernetes.io/url-map",
    "ingress.kubernetes.io/https-forwarding-rule",
    "ingress.kubernetes.io/https-target-proxy",
    "ingress.kubernetes.io/ssl-cert",
    "ingress.kubernetes.io/static-ip",
    "ingress.gcp.kubernetes.io/pre-shared-cert",
    "kubernetes.io/change-cause",
    "pv.kubernetes.io/bind-completed",
    "pv.kubernetes.io/bound-by-controller",
    "volume.beta.kubernetes.io/storage-provisioner",
    "volume.kubernetes.io/storage-provisioner",
    "control-plane.alpha.kubernetes.io/leader",
}

def clean(obj):
    obj.pop("status", None)
    md = obj.get("metadata", {})
    for k in DROP_META:
        md.pop(k, None)
    annos = md.get("annotations")
    if annos:
        for k in list(annos):
            if k in DROP_ANNOS or k.startswith("kubectl.kubernetes.io/"):
                annos.pop(k, None)
        if not annos:
            md.pop("annotations", None)
    labels = md.get("labels")
    if labels:
        for k in list(labels):
            if k.startswith("kubernetes.io/metadata.name"):
                labels.pop(k, None)

    kind = obj.get("kind", "")
    spec = obj.get("spec", {}) or {}
    if kind == "Service":
        for k in ("clusterIP", "clusterIPs", "ipFamilies", "ipFamilyPolicy",
                  "internalTrafficPolicy", "sessionAffinity"):
            spec.pop(k, None)
        if spec.get("type") in ("NodePort", "ClusterIP"):
            for p in spec.get("ports", []):
                if obj["metadata"].get("name") not in ("kbs",):
                    p.pop("nodePort", None)
    if kind == "PersistentVolumeClaim":
        spec.pop("volumeName", None)
        spec.pop("volumeMode", None)
        md.pop("annotations", None)
    if kind in ("Deployment", "StatefulSet", "DaemonSet"):
        spec.pop("revisionHistoryLimit", None)
    # default SA token automount noise
    return obj

def main():
    data = json.load(sys.stdin)
    items = data.get("items", [data])
    out = []
    for it in items:
        it = clean(it)
        out.append(yaml.safe_dump(it, default_flow_style=False, sort_keys=False))
    sys.stdout.write("\n---\n".join(out))

if __name__ == "__main__":
    main()
