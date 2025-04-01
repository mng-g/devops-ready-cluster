# DevOps Ready Cluster

## Overview
DevOps Ready Cluster is a CLI tool written in Go that automates the setup of a Kubernetes cluster using Kind. It provides out-of-the-box support for installing essential DevOps tools such as Metrics Server, Ingress, MetalLB, Cert-Manager, ArgoCD, Prometheus, Loki, and CloudNativePG.

This project is designed for DevOps engineers looking to quickly set up a Kubernetes cluster with production-grade configurations for testing, development, and learning purposes.

## Features
- **Cluster Management**: Create and delete Kubernetes clusters with Kind
- **Metrics & Monitoring**: Install Metrics Server, Prometheus, and Grafana
- **Networking**: Deploy Ingress Controller and MetalLB
- **Security & Certificates**: Set up Cert-Manager for certificate management
- **GitOps**: Install ArgoCD for continuous deployment
- **Logging**: Deploy Grafana Loki for log aggregation
- **Database**: Install CloudNativePG for PostgreSQL management
- **Messaging**: Install Kafka for event streaming

## Prerequisites
Before using DevOps Ready Cluster, ensure you have the following installed:

- [Go](https://go.dev/dl/)
- [Kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)
- [Docker](https://docs.docker.com/get-docker/)

## Installation
```sh
git clone https://github.com/yourusername/devops-ready-cluster.git
cd devops-ready-cluster
go build -o devops-ready-cluster
mv devops-ready-cluster /usr/local/bin/
```

## Usage
### Create a Kubernetes Cluster
```sh
devops-ready-cluster create --name my-cluster
```

### Delete a Kubernetes Cluster
```sh
devops-ready-cluster delete --name my-cluster
```

### Install DevOps Tools
```sh
devops-ready-cluster install metrics-server
devops-ready-cluster install ingress
devops-ready-cluster install metallb
devops-ready-cluster install cert-manager
devops-ready-cluster install argocd
devops-ready-cluster install monitoring
devops-ready-cluster install logging
devops-ready-cluster install database
devops-ready-cluster install kafka
```

## Roadmap
- [ ] Add support for components installation via config file
- [ ] Implement automated TLS setup with Cert-Manager and an internal CA
- [ ] Support for multi-cluster setups
- [ ] Extend Helm chart customizations