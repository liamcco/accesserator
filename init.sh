git init

flox init

flox install kubectl docker kubebuilder kubectx k9s go kind

flox activate 

kubebuilder init --domain kartverket.no --repo github.com/kartverket/accesserator

kubebuilder create api --group accesserator --version v1alpha1 --kind SecurityConfig --resource --controller

make manifests

sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512

kind create cluster --image kindest/node:v1.35.0

kind apply -f cert-manager.yaml --name kind-kind

make install 

make run

## Endring av CRD

make build
make install # Installerer CRD på nytt i Kubernetes

kubectl apply -f examples/example.yaml

## Kjøring av tester
make setup-envtest # Setter opp envtest med nødvendige binærfiler
go test ./... -v -count=1 # Kjører alle tester

kubebuilder create api --group "" --version v1 --kind Pod

kubebuilder create webhook --group "" --version v1 --kind Pod --defaulting --defaulting-path=/mutate-pod

make docker-build docker-push IMG=accesserator:v0.0.5 

kind load docker-image accesserator:v0.0.6 --name kind-kind

make deploy IMG=accesserator:v0.0.6

kubectl apply -f examples/pod.yaml