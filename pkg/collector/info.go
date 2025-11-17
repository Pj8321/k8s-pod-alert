package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Details struct {
	PodName      string
	Namespace    string
	NodeName     string
	Reason       string
	Phase        corev1.PodPhase
	Container    string
	ExitCode     *int32
	FinishedAt   *time.Time
	LastMessage  string
	PrevLogs     string
	Events       string
}

func Gather(ctx context.Context, cs *kubernetes.Clientset, pod *corev1.Pod, containerName string, prevLogs bool) (*Details, error) {
	d := &Details{
		PodName:   pod.Name,
		Namespace: pod.Namespace,
		NodeName:  pod.Spec.NodeName,
		Reason:    pod.Status.Reason,
		Phase:     pod.Status.Phase,
		Container: containerName,
	}

	// Exit code and finishedAt if available
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == containerName {
			if c.LastTerminationState.Terminated != nil {
				e := c.LastTerminationState.Terminated.ExitCode
				d.ExitCode = &e
				t := c.LastTerminationState.Terminated.FinishedAt.Time
				d.FinishedAt = &t
				d.LastMessage = c.LastTerminationState.Terminated.Reason
			}
			break
		}
	}

	// Fetch previous logs (for restarts) or current logs if prev not available
	if prevLogs {
		logStr := fetchLogs(ctx, cs, pod.Namespace, pod.Name, containerName, true)
		if logStr == "" {
			logStr = fetchLogs(ctx, cs, pod.Namespace, pod.Name, containerName, false)
		}
		d.PrevLogs = truncate(logStr, 4000)
	}

	// Events
	evts, _ := cs.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", pod.Name),
	})
	var b strings.Builder
	for _, e := range evts.Items {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", e.LastTimestamp.Format(time.RFC3339), e.Reason, strings.TrimSpace(e.Message))
	}
	d.Events = truncate(b.String(), 4000)

	return d, nil
}

func fetchLogs(ctx context.Context, cs *kubernetes.Clientset, ns, pod, container string, previous bool) string {
	req := cs.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{Container: container, Previous: previous, TailLines: int64Ptr(200)})
	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()
	buf := new(strings.Builder)
	_, _ = buf.ReadFrom(stream)
	return buf.String()
}

func int64Ptr(i int64) *int64 { return &i }

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "\n... (truncated)"
}
