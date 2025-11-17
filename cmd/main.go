// Command entrypoint
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"k8s-pod-alert/pkg/watcher"
)

func main() {
	var kubeconfig string
	var slackWebhook string
	var clusterName string

	flag.StringVar(&kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&slackWebhook, "slack-webhook", os.Getenv("SLACK_WEBHOOK_URL"), "Slack Incoming Webhook URL")
	flag.StringVar(&clusterName, "cluster-name", os.Getenv("CLUSTER_NAME"), "Logical cluster name for alerts")
	flag.Parse()

	if slackWebhook == "" {
		log.Fatal("SLACK_WEBHOOK_URL (or --slack-webhook) is required")
	}
	if clusterName == "" {
		clusterName = "unknown-cluster"
	}

	// Build config: in-cluster first, fallback to kubeconfig
	var cfg *rest.Config
	var err error
	cfg, err = rest.InClusterConfig()
	if err != nil {
		if kubeconfig == "" {
			log.Printf("in-cluster config not found, trying $KUBECONFIG...")
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("failed to build kube config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("failed to create clientset: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("starting k8s-pod-alert at %s\n", time.Now().Format(time.RFC3339))

	watcher.StartPodWatcher(ctx, clientset, watcher.Config{
		SlackWebhook: slackWebhook,
		ClusterName:  clusterName,
		AlertOnRestart: getBoolEnv("ALERT_ON_RESTART", true),
		AlertOnCompletion: getBoolEnv("ALERT_ON_COMPLETION", true),
		AlertOnEviction: getBoolEnv("ALERT_ON_EVICTION", true),
		MuteSeconds: getIntEnv("MUTE_SECONDS", 120),
		IgnoreExitCodeZero: getBoolEnv("IGNORE_EXIT_CODE_ZERO", true),
	})

	<-ctx.Done()
}

func getBoolEnv(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if v == "1" || v == "true" || v == "TRUE" || v == "yes" {
			return true
		}
		if v == "0" || v == "false" || v == "FALSE" || v == "no" {
			return false
		}
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		_, err := fmt.Sscanf(v, "%d", &i)
		if err == nil { return i }
	}
	return def
}
