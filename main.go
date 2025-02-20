package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
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
		color.Red(stderr.String())
		return err
	}
	return nil
}

func log(msg string) {
	color.Cyan(msg)
}

func createCluster(cmd *cobra.Command, args []string) {
	log("Creating Kubernetes cluster with Kind...")
	if err := runCommand("kind", "create", "cluster", "--name", "k8s-playground", "--config", "kind-config.yaml"); err != nil {
		color.Red("Error creating cluster: %v", err)
		os.Exit(1)
	}
}

func deleteCluster(cmd *cobra.Command, args []string) {
	log("Deleting Kubernetes cluster...")
	if err := runCommand("kind", "delete", "cluster", "--name", "k8s-playground"); err != nil {
		color.Red("Error deleting cluster: %v", err)
		os.Exit(1)
	}
}

func installMetricsServer(cmd *cobra.Command, args []string) {
	log("Installing Metrics Server...")
	if err := runCommand("kubectl", "apply", "-f", "components.yaml"); err != nil {
		color.Red("Error installing Metrics Server: %v", err)
		os.Exit(1)
	}
}

func installIngress(cmd *cobra.Command, args []string) {
	log("Installing Ingress Controller...")
	if err := runCommand("kubectl", "apply", "-f", "https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml"); err != nil {
		color.Red("Error installing Ingress Controller: %v", err)
		os.Exit(1)
	}
	time.Sleep(5 * time.Second)
	if err := runCommand("kubectl", "wait", "--namespace", "ingress-nginx", "--for=condition=ready", "pod", "--selector=app.kubernetes.io/component=controller", "--timeout=90s"); err != nil {
		color.Red("Ingress Controller is not ready: %v", err)
		os.Exit(1)
	}
}

func installMetalLB(cmd *cobra.Command, args []string) {
	log("Installing MetalLB...")
	if err := runCommand("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb"); err != nil {
		color.Red("Error adding MetalLB Helm repo: %v", err)
		os.Exit(1)
	}
	if err := runCommand("helm", "install", "metallb", "metallb/metallb", "-n", "metallb-system", "--create-namespace"); err != nil {
		color.Red("Error installing MetalLB: %v", err)
		os.Exit(1)
	}
	time.Sleep(20 * time.Second)
	if err := runCommand("kubectl", "apply", "-f", "metallb-config.yaml"); err != nil {
		color.Red("Error applying MetalLB configuration: %v", err)
		os.Exit(1)
	}
}

func installArgoCD(cmd *cobra.Command, args []string) {
	log("Installing Argo CD...")
	if err := runCommand("kubectl", "create", "namespace", "argocd"); err != nil {
		color.Red("Error creating ArgoCD namespace: %v", err)
	}
	if err := runCommand("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm"); err != nil {
		color.Red("Error adding Argo Helm repo: %v", err)
		os.Exit(1)
	}
	if err := runCommand("helm", "install", "argocd-demo", "argo/argo-cd", "-f", "argocd-custom-values.yaml", "-n", "argocd"); err != nil {
		color.Red("Error installing ArgoCD: %v", err)
		os.Exit(1)
	}
}

func installDemoApp(cmd *cobra.Command, args []string) {
	log("Deploying ArgoCD demo app...")
	if err := runCommand("kubectl", "apply", "-f", "argocd-demo-app.yaml"); err != nil {
		color.Red("Error deploying demo app: %v", err)
		os.Exit(1)
	}
}

func installAll(cmd *cobra.Command, args []string) {
	installMetricsServer(cmd, args)
	installIngress(cmd, args)
	installMetalLB(cmd, args)
	installArgoCD(cmd, args)
	installDemoApp(cmd, args)
}

func main() {
	var rootCmd = &cobra.Command{Use: "k8s-cli"}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(&cobra.Command{Use: "create-cluster", Short: "Create Kind Kubernetes cluster", Run: createCluster})
	rootCmd.AddCommand(&cobra.Command{Use: "delete-cluster", Short: "Delete Kind Kubernetes cluster", Run: deleteCluster})
	rootCmd.AddCommand(&cobra.Command{Use: "install-metrics", Short: "Install Metrics Server", Run: installMetricsServer})
	rootCmd.AddCommand(&cobra.Command{Use: "install-ingress", Short: "Install Ingress Controller", Run: installIngress})
	rootCmd.AddCommand(&cobra.Command{Use: "install-metallb", Short: "Install MetalLB", Run: installMetalLB})
	rootCmd.AddCommand(&cobra.Command{Use: "install-argocd", Short: "Install Argo CD", Run: installArgoCD})
	rootCmd.AddCommand(&cobra.Command{Use: "install-demo", Short: "Install demo application", Run: installDemoApp})
	rootCmd.AddCommand(&cobra.Command{Use: "install-all", Short: "Install all components", Run: installAll})

	if err := rootCmd.Execute(); err != nil {
		color.Red("Error executing command: %v", err)
		os.Exit(1)
	}
}
