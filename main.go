package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var verbose bool

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if verbose {
		fmt.Println(stdout.String())
	}
	if err != nil {
		fmt.Println(stderr.String())
		return err
	}
	return nil
}

func logInfo(msg string) {
	fmt.Println("[INFO]", msg)
}

func logWarning(msg string) {
	fmt.Println("[WARNING]", msg)
}

func logError(msg string) {
	fmt.Println("[ERROR]", msg)
}

func logFatal(msg string, err error) {
	logError(msg + ": " + err.Error())
	os.Exit(1)
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(dest, data, 0644)
}

func fileContains(filePath, searchStr string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if bytes.Contains(scanner.Bytes(), []byte(searchStr)) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func extractAddressRange(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- ") { // Look for address range
			return strings.TrimPrefix(line, "- "), nil
		}
	}

	return "", scanner.Err()
}

func getClusters(cmd *cobra.Command, args []string) {
	logInfo("Getting Kubernetes clusters with Kind...")

	output, err := exec.Command("kind", "get", "clusters").Output()
	if err != nil {
		logError("Error listing clusters: " + err.Error())
		os.Exit(1)
	}

	clusters := strings.TrimSpace(string(output))
	if clusters == "" {
		logInfo("No Kind clusters found.")
	} else {
		logInfo("Found clusters:\n" + clusters)
	}
}

func createCluster(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		logError("Cluster name is required (--name)")
		os.Exit(1)
	}
	logInfo("Creating Kubernetes cluster with Kind...")
	if err := runCommand("kind", "create", "cluster", "--name", name, "--config", "kind-config.yaml"); err != nil {
		logError("Error creating cluster: " + err.Error())
		os.Exit(1)
	}
	logInfo("Cluster " + name + " created successfully!")
}

func deleteCluster(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		logError("Cluster name is required (--name)")
		os.Exit(1)
	}
	logInfo("Deleting Kubernetes cluster with Kind...")
	if err := runCommand("kind", "delete", "cluster", "--name", name); err != nil {
		logError("Error deleting cluster: " + err.Error())
		os.Exit(1)
	}
	logInfo("Cluster " + name + " deleted successfully!")
}

func installMetricsServer(cmd *cobra.Command, args []string) {
	filePath := "components.yaml"

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		logInfo("Downloading Metrics Server components.yaml...")

		if err := downloadFile("https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml", filePath); err != nil {
			logError("Failed to download components.yaml: " + err.Error())
			os.Exit(1)
		}

		if contains, err := fileContains(filePath, "--kubelet-insecure-tls"); err != nil {
			logError("Error reading components.yaml: " + err.Error())
			os.Exit(1)
		} else if contains {
			logInfo("components.yaml already contains --kubelet-insecure-tls")
			logInfo("Skipping modification.")
		} else {
			logWarning("The Metrics Server requires a modification to the components.yaml file.")
			logWarning("Please add the argument `- --kubelet-insecure-tls` after `- --kubelet-use-node-status-port` in components.yaml.")
			logWarning("Press Enter to continue...")
			fmt.Scanln()
			logInfo("Continuing execution...")
		}
	}

	logInfo("Installing Metrics Server...")
	if err := runCommand("kubectl", "apply", "-f", filePath); err != nil {
		logError("Error installing Metrics Server: " + err.Error())
		os.Exit(1)
	}
	logInfo("Metrics Server installed successfully!")
}

func installIngress(cmd *cobra.Command, args []string) {
	logInfo("Installing Ingress Controller...")
	if err := runCommand("kubectl", "apply", "-f", "https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml"); err != nil {
		logError("Error installing Ingress Controller: " + err.Error())
		os.Exit(1)
	}
	time.Sleep(5 * time.Second)
	if err := runCommand("kubectl", "wait", "--namespace", "ingress-nginx", "--for=condition=ready", "pod", "--selector=app.kubernetes.io/component=controller", "--timeout=90s"); err != nil {
		logError("Ingress Controller is not ready: " + err.Error())
		os.Exit(1)
	}
	logInfo("Ingress Controller installed successfully!")
}

func installMetalLB(cmd *cobra.Command, args []string) {
	logInfo("Installing MetalLB...")

	if err := runCommand("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb"); err != nil {
		logError("Error adding MetalLB Helm repo" + err.Error())
	}

	if err := runCommand("helm", "install", "metallb", "metallb/metallb", "-n", "metallb-system", "--create-namespace"); err != nil {
		logError("Error installing MetalLB" + err.Error())
	}

	time.Sleep(30 * time.Second) // Ensure MetalLB is ready before applying config

	addressRange, err := extractAddressRange("metallb-config.yaml")
	if err != nil {
		logError("Error reading MetalLB configuration file" + err.Error())
	}

	logWarning(fmt.Sprintf("Are you sure you want to use the address range %s?", addressRange))
	logWarning("If not, edit the metallb-config.yaml file before pressing Enter.")
	fmt.Scanln()
	logInfo("Continuing installation...")

	if err := runCommand("kubectl", "apply", "-f", "metallb-config.yaml"); err != nil {
		logError("Error applying MetalLB configuration" + err.Error())
	}
	logInfo("MetalLB installed successfully!")
}

// TODO: Create issuer for self-signed certificates and interal CA
func installCertManager(cmd *cobra.Command, args []string) {
	logInfo("Installing Cert-Manager...")

	if err := runCommand("helm", "repo", "add", "jetstack", "https://charts.jetstack.io", "--force-update"); err != nil {
		logError("Error adding Jetstack Helm repo: " + err.Error())
		os.Exit(1)
	}

	if err := runCommand(
		"helm", "install", "cert-manager", "jetstack/cert-manager",
		"--namespace", "cert-manager",
		"--create-namespace",
		"--set", "crds.enabled=true",
		"--set", "extraArgs={--dns01-recursive-nameservers-only,--dns01-recursive-nameservers=8.8.8.8:53,1.1.1.1:53}",
	); err != nil {
		logError("Error installing Cert-Manager: " + err.Error())
		os.Exit(1)
	}

	logInfo("Cert-Manager installation initiated. Waiting for readiness check...")

	if err := runCommand(
		"kubectl", "wait", "--namespace", "cert-manager",
		"--for=condition=ready", "pod", "--selector=app.kubernetes.io/name=cert-manager",
		"--timeout=90s",
	); err != nil {
		logError("Cert-Manager is not ready: " + err.Error())
		os.Exit(1)
	}

	logInfo("Cert-Manager installation completed successfully!")
}

func installArgoCD(cmd *cobra.Command, args []string) {
	logInfo("Installing Argo CD...")

	// Add Argo Helm repository
	if err := runCommand("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm"); err != nil {
		logFatal("Error adding Argo Helm repo", err)
	}

	// Install ArgoCD with custom values
	if err := runCommand("helm", "install", "argocd", "argo/argo-cd", "-f", "argocd-custom-values.yaml", "-n", "argocd", "--create-namespace"); err != nil {
		logFatal("Error installing ArgoCD", err)
	}

	logInfo("ArgoCD installation initiated. Waiting for readiness check...")

	// Wait for ArgoCD server to be ready
	if err := runCommand("kubectl", "wait", "--namespace", "argocd",
		"--for=condition=available", "deployment/argocd-server", "--timeout=90s"); err != nil {
		logError("ArgoCD server is not ready yet: " + err.Error())
	}

	// TODO: add TLS certificates for ArgoCD created by cert-manager. Use internal CA for now.

	// Inform user about domain and certificate settings
	logInfo("ArgoCD installation completed successfully!")
	logInfo("ArgoCD is accessible at: https://argocd.local")
	logWarning("Ensure that 'argocd.local' resolves to the correct IP by:")
	logWarning("1. Editing your /etc/hosts file")
	logWarning("2. Configuring DNS correctly")
	logWarning("3. Modifying 'argocd-custom-values.yaml' to use a different domain if needed")

	// Provide initial admin password retrieval command
	logInfo("To retrieve the initial admin password, run:")
	logInfo(`kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`)
}

// TODO: Create an ingress for Grafana and Prometheus
func installMonitoring(cmd *cobra.Command, args []string) {
	logInfo("Installing Prometheus and Grafana monitoring stack...")

	if err := runCommand("helm", "repo", "add", "prometheus-community", "https://prometheus-community.github.io/helm-charts"); err != nil {
		logFatal("Error adding Prometheus Helm repo", err)
	}

	if err := runCommand("helm", "repo", "update"); err != nil {
		logFatal("Error updating Helm repositories", err)
	}

	if err := runCommand(
		"helm", "install", "prometheus-stack", "prometheus-community/kube-prometheus-stack",
		"--namespace", "monitoring",
		"--create-namespace",
	); err != nil {
		logFatal("Error installing Prometheus stack", err)
	}

	logInfo("✅ Prometheus and Grafana installed successfully!")

	logInfo("\n🔹 **Access Dashboards:**")

	logInfo("📊 **Prometheus Dashboard:** http://localhost:9090")
	logInfo("Run the following command to forward the Prometheus service:")
	logInfo("kubectl port-forward svc/prometheus-stack-kube-prom-prometheus -n monitoring 9090:9090")

	logInfo("\n📈 **Grafana Dashboard:** http://localhost:3000")
	logInfo("Run the following commands to forward the Grafana service:")
	logInfo(`export POD_NAME=$(kubectl --namespace monitoring get pod -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=prometheus-stack" -o name)`)
	logInfo("kubectl --namespace monitoring port-forward $POD_NAME 3000:3000")

	logInfo("\n🔑 **Retrieve the Grafana admin password:**")
	logInfo(`kubectl --namespace monitoring get secrets prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d ; echo`)
}

func installLogging(cmd *cobra.Command, args []string) {
	logInfo("Installing Grafana Loki for logging...")

	if err := runCommand("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts"); err != nil {
		logFatal("Error adding Grafana Helm repo", err)
	}

	if err := runCommand("helm", "repo", "update"); err != nil {
		logFatal("Error updating Helm repositories", err)
	}

	if err := runCommand(
		"helm", "upgrade", "--install", "loki", "grafana/loki-stack",
		"--namespace", "logging",
		"--create-namespace",
		"--set", "loki.enabled=true",
		"--set", "promtail.enabled=true",
		"--set", "promtail.config.server.http_listen_port=9080",
		"--set", "promtail.config.server.grpc_listen_port=0",
	); err != nil {
		logFatal("Error installing Loki stack", err)
	}

	logInfo("Grafana Loki installed successfully!")
	logInfo("To check logs, run:")
	logInfo(`kubectl -n logging logs -l app.kubernetes.io/name=promtail`)
}

func installDatabase(cmd *cobra.Command, args []string) {
	logInfo("Installing CloudNativePG database...")

	if err := runCommand("kubectl", "apply", "--server-side", "-f", "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.25/releases/cnpg-1.25.1.yaml"); err != nil {
		logFatal("Error applying CloudNativePG manifests", err)
	}

	logInfo("CloudNativePG installed successfully!")
	logWarning("To manage CloudNativePG more easily, install the cnpg plugin:")
	logWarning(`curl -sSfL https://github.com/cloudnative-pg/cloudnative-pg/raw/main/hack/install-cnpg-plugin.sh | sudo sh -s -- -b /usr/local/bin`)
	logInfo("Once installed, you can check the PostgreSQL cluster status with:")
	logInfo(`kubectl cnpg status <CNPG_CLUSTER> -n <NAMESPACE>`)
}

func installKafka(cmd *cobra.Command, args []string) {
	logInfo("Installing Kafka...")

	if err := runCommand(
		"helm", "install", "strimzi-cluster-operator", "oci://quay.io/strimzi-helm/strimzi-kafka-operator",
		"--create-namespace", "--namespace", "kafka",
		"--set", "replicas=2",
	); err != nil {
		logFatal("Error installing Kafka", err)
	}

	if err := runCommand("kubectl", "wait", "--namespace", "kafka", "--for=condition=ready", "pod", "--selector=name=strimzi-cluster-operator", "--timeout=90s"); err != nil {
		logError("Ingress Controller is not ready: " + err.Error())
		os.Exit(1)
	}

	logInfo("Kafka installed successfully!")
	logInfo("To deply a Kafka cluster, run:")
	logInfo("kubectl apply -f https://strimzi.io/examples/latest/kafka/kraft/kafka-single-node.yaml -n kafka")
	logInfo("To produce messages, run:")
	logInfo("kubectl -n kafka run kafka-producer -ti --image=quay.io/strimzi/kafka:0.45.0-kafka-3.9.0 --rm=true --restart=Never -- bin/kafka-console-producer.sh --bootstrap-server my-cluster-kafka-bootstrap:9092 --topic my-topic")
	logInfo("To consume messages, run:")
	logInfo("kubectl -n kafka run kafka-consumer -ti --image=quay.io/strimzi/kafka:0.45.0-kafka-3.9.0 --rm=true --restart=Never -- bin/kafka-console-consumer.sh --bootstrap-server my-cluster-kafka-bootstrap:9092 --topic my-topic --from-beginning")
	logInfo("To delete the Kafka cluster, run:")
	logInfo("kubectl delete kafka my-cluster -n kafka")
}

// TODO: use helm to deploy a release and inform the user about the URL exposed via ingress
func installDemoApp(cmd *cobra.Command, args []string) {
	logInfo("Deploying ArgoCD demo app...")
	if err := runCommand("kubectl", "apply", "-f", "argocd-demo-app.yaml"); err != nil {
		logError("Error deploying demo app: " + err.Error())
		os.Exit(1)
	}
	logInfo("Demo app deployed successfully!")
}

func installAll(cmd *cobra.Command, args []string) {
	installMetricsServer(cmd, args)
	installIngress(cmd, args)
	installMetalLB(cmd, args)
	installCertManager(cmd, args)
	installArgoCD(cmd, args)
	installDatabase(cmd, args)
	installKafka(cmd, args)
	installMonitoring(cmd, args)
	installLogging(cmd, args)
}

func main() {
	var rootCmd = &cobra.Command{Use: "devops-ready-cluster"}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	getCmd := &cobra.Command{Use: "get-clusters", Short: "Get Kind Kubernetes cluster", Run: getClusters}

	createCmd := &cobra.Command{Use: "create-cluster", Short: "Create Kind Kubernetes cluster", Run: createCluster}
	createCmd.Flags().String("name", "", "Cluster name (required)")
	createCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{Use: "delete-cluster", Short: "Delete Kind Kubernetes cluster", Run: deleteCluster}
	deleteCmd.Flags().String("name", "", "Cluster name (required)")
	deleteCmd.MarkFlagRequired("name")

	rootCmd.AddCommand(getCmd, createCmd, deleteCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "install-metrics", Short: "Install Metrics Server", Run: installMetricsServer})
	rootCmd.AddCommand(&cobra.Command{Use: "install-ingress", Short: "Install Ingress Controller", Run: installIngress})
	rootCmd.AddCommand(&cobra.Command{Use: "install-metallb", Short: "Install MetalLB", Run: installMetalLB})
	rootCmd.AddCommand(&cobra.Command{Use: "install-cert-manager", Short: "Install Cert-Manager", Run: installCertManager})
	rootCmd.AddCommand(&cobra.Command{Use: "install-argocd", Short: "Install Argo CD", Run: installArgoCD})
	rootCmd.AddCommand(&cobra.Command{Use: "install-monitoring", Short: "Install Monitoring Stack", Run: installMonitoring})
	rootCmd.AddCommand(&cobra.Command{Use: "install-logging", Short: "Install Logging Stack", Run: installLogging})
	rootCmd.AddCommand(&cobra.Command{Use: "install-database", Short: "Install CloudNativePG Database", Run: installDatabase})
	rootCmd.AddCommand(&cobra.Command{Use: "install-kafka", Short: "Install Kafka", Run: installKafka})
	rootCmd.AddCommand(&cobra.Command{Use: "install-demo", Short: "Install demo application", Run: installDemoApp})
	rootCmd.AddCommand(&cobra.Command{Use: "install-all", Short: "Install all components", Run: installAll})

	if err := rootCmd.Execute(); err != nil {
		logError("Error executing command: " + err.Error())
		os.Exit(1)
	}
}
