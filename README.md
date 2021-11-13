# cluster-api-provider-civo
capc (cap ka) is a cluster api provider for the civo platform created for the hackathon for fun! Interested in helping drive it forward? you are more then welcome to join in!

## Run locally

To run this Cluster API with this provider locally:

1. Install `clusterctl`: `curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.0.1/clusterctl-linux-amd64 -o clusterctl`
2. Create `kind` cluster: `kind create cluster`
3. Save kubeconfig from `kind` and export it: `kind get kubeconfig > /tmp/kubeconfig && export KUBECONFIG=/tmp/kubeconfig`
4. Initialize Cluster API controller: `clusterctl init`
5. Install CRDs: `kubectl apply -f ./config/samples/crds.yaml`
6. Export your Civo API key: `export CIVO_API_KEY=your_key`
7. Run provider controller: `make run`
8. Apply sample manifest with cluster definition: `kubectl apply -f ./config/samples/cluster.yaml`
