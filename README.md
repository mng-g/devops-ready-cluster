# devops-ready-cluster

A simple tool written in Go to create and set up a gentle cluster, ready for a demo.

### Dependencies:
kind, helm

### Usage:
```bash
go run main.go create-cluster --name k8s-playground
go run main.go install-all
go run main.go install-demo # deploy a simple nginx app using argocd

# To the the argocd initial password (user: admin):
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

# To remove the cluster:
go run main.go delete-cluster --name k8s-playground
```