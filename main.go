package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

func logError(msg string) {
	fmt.Println("[ERROR]", msg)
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
}

// KubernetesResource is a generic representation of a Kubernetes manifest
type KubernetesResource struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
}

// modifyMetricsServerFile ensures the required flag is present in the Deployment
func modifyMetricsServerFile(filePath string) error {
	// Read the YAML file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Parse the YAML as a list of Kubernetes objects
	var resources []KubernetesResource
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var res KubernetesResource
		if err := decoder.Decode(&res); err != nil {
			break
		}
		resources = append(resources, res)
	}

	// Find the "metrics-server" Deployment
	for i, res := range resources {
		if res.Kind == "Deployment" && res.Metadata["name"] == "metrics-server" {
			// Navigate to spec.template.spec.containers
			specTemplate, ok := res.Spec["template"].(map[string]interface{})
			if !ok {
				continue
			}
			spec, ok := specTemplate["spec"].(map[string]interface{})
			if !ok {
				continue
			}
			containers, ok := spec["containers"].([]interface{})
			if !ok || len(containers) == 0 {
				continue
			}

			// Modify the args for the first container
			container := containers[0].(map[string]interface{})
			args, ok := container["args"].([]interface{})
			if !ok {
				args = []interface{}{}
			}

			// Check if --kubelet-insecure-tls already exists
			found := false
			for _, arg := range args {
				if arg == "--kubelet-insecure-tls" {
					found = true
					break
				}
			}

			// Add the missing flag if necessary
			if !found {
				args = append(args, "--kubelet-insecure-tls")
				container["args"] = args
				logInfo("Added --kubelet-insecure-tls to metrics-server deployment")
			} else {
				logInfo("--kubelet-insecure-tls already present")
			}

			// Update the modified resource back in the list
			resources[i] = res
			break
		}
	}

	// Convert back to YAML
	modifiedData, err := yaml.Marshal(resources)
	if err != nil {
		return fmt.Errorf("failed to marshal modified YAML: %v", err)
	}

	// Write back to the file
	if err := ioutil.WriteFile(filePath, modifiedData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	log.Println("[INFO] Successfully modified components.yaml")
	return nil
}

func installMetricsServer(cmd *cobra.Command, args []string) {
	filePath := "components.yaml"
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		logInfo("Downloading Metrics Server components.yaml...")
		resp, err := http.Get("https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")
		if err != nil {
			logError("Failed to download components.yaml: " + err.Error())
			os.Exit(1)
		}
		defer resp.Body.Close()
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logError("Failed to read downloaded components.yaml: " + err.Error())
			os.Exit(1)
		}
		ioutil.WriteFile(filePath, data, 0644)

		// TODO: EDIT THE FILE
		fmt.Println("Add to the components.yaml file the arg - --kubelet-insecure-tls after - --kubelet-use-node-status-port. Then press Enter to continue...")
		fmt.Scanln() // Waits for user input (Enter key)
		fmt.Println("Continuing execution...")

		// if err := modifyMetricsServerFile(filePath); err != nil {
		// 	logError("Error modifying components.yaml: " + err.Error())
		// 	os.Exit(1)
		// }
	}

	logInfo("Installing Metrics Server...")
	if err := runCommand("kubectl", "apply", "-f", filePath); err != nil {
		logError("Error installing Metrics Server: " + err.Error())
		os.Exit(1)
	}
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
}

func installMetalLB(cmd *cobra.Command, args []string) {
	logInfo("Installing MetalLB...")
	if err := runCommand("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb"); err != nil {
		logError("Error adding MetalLB Helm repo: " + err.Error())
		os.Exit(1)
	}
	if err := runCommand("helm", "install", "metallb", "metallb/metallb", "-n", "metallb-system", "--create-namespace"); err != nil {
		logError("Error installing MetalLB: " + err.Error())
		os.Exit(1)
	}
	time.Sleep(30 * time.Second)
	if err := runCommand("kubectl", "apply", "-f", "metallb-config.yaml"); err != nil {
		logError("Error applying MetalLB configuration: " + err.Error())
		os.Exit(1)
	}
}

func installArgoCD(cmd *cobra.Command, args []string) {
	logInfo("Installing Argo CD...")
	if err := runCommand("kubectl", "create", "namespace", "argocd"); err != nil {
		logError("Error creating ArgoCD namespace: " + err.Error())
	}
	if err := runCommand("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm"); err != nil {
		logError("Error adding Argo Helm repo: " + err.Error())
		os.Exit(1)
	}
	// added override values for argocd to enable imageUpdater
	if err := runCommand("helm", "install", "argocd", "argo/argo-cd", "-f", "argocd-custom-values.yaml", "-n", "argocd"); err != nil {
		logError("Error installing ArgoCD: " + err.Error())
		os.Exit(1)
	}
}

func installDemoApp(cmd *cobra.Command, args []string) {
	logInfo("Deploying ArgoCD demo app...")
	if err := runCommand("kubectl", "apply", "-f", "argocd-demo-app.yaml"); err != nil {
		logError("Error deploying demo app: " + err.Error())
		os.Exit(1)
	}
}

func installAll(cmd *cobra.Command, args []string) {
	installMetricsServer(cmd, args)
	installIngress(cmd, args)
	installMetalLB(cmd, args)
	installArgoCD(cmd, args)
}

func main() {
	var rootCmd = &cobra.Command{Use: "k8s-cli"}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	createCmd := &cobra.Command{Use: "create-cluster", Short: "Create Kind Kubernetes cluster", Run: createCluster}
	createCmd.Flags().String("name", "", "Cluster name (required)")
	createCmd.MarkFlagRequired("name")

	deleteCmd := &cobra.Command{Use: "delete-cluster", Short: "Delete Kind Kubernetes cluster", Run: deleteCluster}
	deleteCmd.Flags().String("name", "", "Cluster name (required)")
	deleteCmd.MarkFlagRequired("name")

	rootCmd.AddCommand(createCmd, deleteCmd)
	rootCmd.AddCommand(&cobra.Command{Use: "install-metrics", Short: "Install Metrics Server", Run: installMetricsServer})
	rootCmd.AddCommand(&cobra.Command{Use: "install-ingress", Short: "Install Ingress Controller", Run: installIngress})
	rootCmd.AddCommand(&cobra.Command{Use: "install-metallb", Short: "Install MetalLB", Run: installMetalLB})
	rootCmd.AddCommand(&cobra.Command{Use: "install-argocd", Short: "Install Argo CD", Run: installArgoCD})
	rootCmd.AddCommand(&cobra.Command{Use: "install-demo", Short: "Install demo application", Run: installDemoApp})
	rootCmd.AddCommand(&cobra.Command{Use: "install-all", Short: "Install all components", Run: installAll})

	if err := rootCmd.Execute(); err != nil {
		logError("Error executing command: " + err.Error())
		os.Exit(1)
	}
}
