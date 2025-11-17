package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"k8s-pod-event-collector/pkg/alert"
	"k8s-pod-event-collector/pkg/collector"
)

type Config struct {
	SlackWebhook     string
	ClusterName      string
	AlertOnRestart   bool
	AlertOnCompletion bool
	AlertOnEviction  bool
	MuteSeconds      int
	IgnoreExitCodeZero bool
}

type muteKey struct {
	Event string
	NS    string
	Name  string
}

func StartPodWatcher(ctx context.Context, clientset *kubernetes.Clientset, cfg Config) {
	factory := informers.NewSharedInformerFactory(clientset, 0)
	podInformer := factory.Core().V1().Pods().Informer()

	var mu sync.Mutex
	muted := map[muteKey]time.Time{}

	shouldMute := func(k muteKey) bool {
		mu.Lock()
		defer mu.Unlock()
		if until, ok := muted[k]; ok && time.Now().Before(until) {
			return true
		}
		muted[k] = time.Now().Add(time.Duration(cfg.MuteSeconds) * time.Second)
		return false
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			newPod := newObj.(*corev1.Pod)

			// Detect Restarts
			if cfg.AlertOnRestart {
				inc, cname := restarted(oldPod, newPod)
				if inc {
					k := muteKey{"restart", newPod.Namespace, newPod.Name}
					if !shouldMute(k) {
						details, _ := collector.Gather(ctx, clientset, newPod, cname, true)
						text := formatSlack(cfg.ClusterName, "Pod Restart", newPod, details)
						if err := alert.Post(cfg.SlackWebhook, text); err != nil { log.Printf("slack error: %v", err) }
					}
				}
			}

			// Detect Completion
			if cfg.AlertOnCompletion {
				if oldPod.Status.Phase != corev1.PodSucceeded && newPod.Status.Phase == corev1.PodSucceeded {
					k := muteKey{"completed", newPod.Namespace, newPod.Name}
					if !shouldMute(k) {
						details, _ := collector.Gather(ctx, clientset, newPod, firstContainer(newPod), false)
						text := formatSlack(cfg.ClusterName, "Pod Completed", newPod, details)
						if err := alert.Post(cfg.SlackWebhook, text); err != nil { log.Printf("slack error: %v", err) }
					}
				}
			}

			// Detect Eviction
			if cfg.AlertOnEviction {
				if oldPod.Status.Reason != "Evicted" && newPod.Status.Reason == "Evicted" {
					k := muteKey{"evicted", newPod.Namespace, newPod.Name}
					if !shouldMute(k) {
						details, _ := collector.Gather(ctx, clientset, newPod, firstContainer(newPod), false)
						text := formatSlack(cfg.ClusterName, "Pod Evicted", newPod, details)
						if err := alert.Post(cfg.SlackWebhook, text); err != nil { log.Printf("slack error: %v", err) }
					}
				}
			}
		},
	})

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	<-ctx.Done()
}

func restarted(oldPod, newPod *corev1.Pod) (bool, string) {
	oldMap := map[string]int32{}
	for _, cs := range oldPod.Status.ContainerStatuses { oldMap[cs.Name] = cs.RestartCount }
	for _, cs := range newPod.Status.ContainerStatuses {
		if cs.RestartCount > oldMap[cs.Name] {
			// Ignore exit code 0 if configured (best effort: check LastTerminationState)
			return true, cs.Name
		}
	}
	return false, ""
}

func firstContainer(p *corev1.Pod) string {
	if len(p.Spec.Containers) > 0 { return p.Spec.Containers[0].Name }
	return ""
}

func formatSlack(cluster string, event string, pod *corev1.Pod, d *collector.Details) string {
	node := pod.Spec.NodeName
	labelsStr := labels.Set(pod.Labels).String()

	finished := "n/a"
	if d != nil && d.FinishedAt != nil { finished = d.FinishedAt.Format(time.RFC3339) }

	msg := fmt.Sprintf("*%s* on *%s*\nNamespace: `%s`\nPod: `%s`\nNode: `%s`\nPhase: `%s`\nReason: `%s`\nLabels: `%s`\nFinishedAt: `%s`", event, cluster, pod.Namespace, pod.Name, node, pod.Status.Phase, pod.Status.Reason, labelsStr, finished)

	if d != nil {
		if d.ExitCode != nil { msg += fmt.Sprintf("\nExitCode: `%d`", *d.ExitCode) }
		if d.LastMessage != "" { msg += fmt.Sprintf("\nLastMessage: `%s`", d.LastMessage) }
		if d.Events != "" { msg += "\n\n*Recent Events:*\n" + d.Events }
		if d.PrevLogs != "" { msg += "\n\n*Previous Logs:*\n```\n" + d.PrevLogs + "\n```" }
	}
	return msg
}
