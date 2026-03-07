package k8s

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/x"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config for kubernetes executor.
type Config struct {
	Kubeconfig    string `json:"kubeconfig,omitempty" yaml:"kubeconfig"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	Image         string `json:"image" yaml:"image"`
	CPURequest    string `json:"cpu_request,omitempty" yaml:"cpu_request"`
	MemoryRequest string `json:"memory_request,omitempty" yaml:"memory_request"`
}

func init() {
	executor.Register("kubernetes", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		var c Config
		if err := cfg.Unmarshal(&c); err != nil {
			return nil, err
		}
		return New(c)
	}, "k8s")
}

type k8sExecutor struct {
	client    kubernetes.Interface
	namespace string
	image     string
	logger    *slog.Logger

	mu            sync.Mutex
	sessions      map[string]string // runID -> podName
	sessionMsgCnt map[string]int    // runID -> message count
	kubeconfig    string            // for kubectl CLI

	cpuRequest    string
	memoryRequest string
}

func New(cfg Config) (executor.Executor, error) {
	var restCfg *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		restCfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("k8s config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	ns := cfg.Namespace
	if ns == "" {
		ns = "agentbox"
	}

	cpuReq := cfg.CPURequest
	if cpuReq == "" {
		cpuReq = "500m"
	}
	memReq := cfg.MemoryRequest
	if memReq == "" {
		memReq = "512Mi"
	}

	return &k8sExecutor{
		client:        client,
		namespace:     ns,
		image:         cfg.Image,
		logger:        slog.Default(),
		sessions:      make(map[string]string),
		sessionMsgCnt: make(map[string]int),
		kubeconfig:    cfg.Kubeconfig,
		cpuRequest:    cpuReq,
		memoryRequest: memReq,
	}, nil
}

func (e *k8sExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	jobName := fmt.Sprintf("abox-%s", req.ID)

	envVars := []corev1.EnvVar{
		{Name: "AGENTBOX_RUN_ID", Value: req.ID},
		{Name: "AGENTBOX_AGENT_FILE", Value: req.AgentFile},
	}
	for k, v := range req.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	image := req.Image
	if image == "" {
		image = e.image
	}

	backoffLimit := int32(0)
	ttl := int32(300)
	labels := map[string]string{
		"app":          "agentbox",
		"agentbox/run": req.ID,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: e.namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "agent",
						Image:   image,
						Env:     envVars,
						Command: []string{"/opt/agentbox/entrypoint.sh"},
					}},
				},
			},
		},
	}

	e.logger.Info("creating k8s job", "name", jobName, "image", image)

	if _, err := e.client.BatchV1().Jobs(e.namespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	return e.waitForJob(ctx, jobName, time.Duration(req.Timeout)*time.Second)
}

func (e *k8sExecutor) waitForJob(ctx context.Context, name string, timeout time.Duration) (*executor.Response, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, fmt.Errorf("job %s timed out", name)
		case <-ticker.C:
			job, err := e.client.BatchV1().Jobs(e.namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("get job: %w", err)
			}
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					logs, _ := e.Logs(ctx, name)
					return &executor.Response{ExitCode: 0, Output: logs}, nil
				}
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					logs, _ := e.Logs(ctx, name)
					return &executor.Response{ExitCode: 1, Output: logs}, nil
				}
			}
		}
	}
}

// Logs gets pod logs by the agentbox/run label.
func (e *k8sExecutor) Logs(ctx context.Context, id string) (string, error) {
	pods, err := e.client.CoreV1().Pods(e.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("agentbox/run=%s", id),
	})
	if err != nil {
		return "", fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for run %s", id)
	}

	logOpts := &corev1.PodLogOptions{
		Container: "agent",
	}
	req := e.client.CoreV1().Pods(e.namespace).GetLogs(pods.Items[0].Name, logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(stream); err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func (e *k8sExecutor) Stop(ctx context.Context, id string) error {
	jobName := fmt.Sprintf("abox-%s", id)
	propagation := metav1.DeletePropagationForeground
	return e.client.BatchV1().Jobs(e.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}

// StartSession creates a long-running Pod for interactive session mode.
func (e *k8sExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	podName := fmt.Sprintf("abox-%s", req.ID)

	image := req.Image
	if image == "" {
		image = e.image
	}

	envVars := []corev1.EnvVar{
		{Name: "AGENTBOX_RUN_ID", Value: req.ID},
		{Name: "AGENTBOX_AGENT_FILE", Value: req.AgentFile},
		{Name: "AGENTBOX_MODE", Value: "session"},
	}
	for k, v := range req.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Pass through ABOX_WEBDAV_URL if set
	if webdavURL := os.Getenv("ABOX_WEBDAV_URL"); webdavURL != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "ABOX_WEBDAV_URL", Value: webdavURL})
	}

	labels := map[string]string{
		"app":            "agentbox",
		"agentbox/run":   req.ID,
		"agentbox/mode":  "session",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: e.namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "agent",
				Image:   image,
				Env:     envVars,
				Command: []string{"/opt/agentbox/entrypoint.sh"},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(e.cpuRequest),
						corev1.ResourceMemory: resource.MustParse(e.memoryRequest),
					},
				},
			}},
		},
	}

	e.logger.Info("creating session pod", "name", podName, "image", image)

	if _, err := e.client.CoreV1().Pods(e.namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return "", fmt.Errorf("create pod: %w", err)
	}

	// Wait for pod to be Running
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			// Clean up the pod on timeout
			_ = e.client.CoreV1().Pods(e.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			return "", fmt.Errorf("pod %s did not become Running within 60s", podName)
		case <-ticker.C:
			p, err := e.client.CoreV1().Pods(e.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return "", fmt.Errorf("get pod: %w", err)
			}
			if p.Status.Phase == corev1.PodRunning {
				e.mu.Lock()
				e.sessions[req.ID] = podName
				e.mu.Unlock()

				e.logger.Info("session pod running", "name", podName)
				return podName, nil
			}
			if p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
				return "", fmt.Errorf("pod %s entered %s phase unexpectedly", podName, p.Status.Phase)
			}
		}
	}
}

// SendMessage executes claude -p in a running session pod via kubectl exec.
func (e *k8sExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	e.mu.Lock()
	podName, ok := e.sessions[id]
	msgCnt := e.sessionMsgCnt[id]
	e.sessionMsgCnt[id] = msgCnt + 1
	e.mu.Unlock()

	if !ok {
		podName = fmt.Sprintf("abox-%s", id)
	}

	args := e.kubectlArgs("exec", podName, "-c", "agent", "--")
	args = append(args,
		"env", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
		"PATH=/home/agent/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)
	args = append(args, "claude", "-p", "--dangerously-skip-permissions")
	if msgCnt > 0 {
		args = append(args, "--continue")
	}
	args = append(args, message)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if e.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+e.kubeconfig)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Info("sending message to session", "pod", podName)

	if err := cmd.Run(); err != nil {
		output := stdout.String()
		if stderr.Len() > 0 {
			output += "\n" + stderr.String()
		}
		return output, fmt.Errorf("kubectl exec: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// SendMessageStream executes claude -p with stream-json output in a running session pod.
func (e *k8sExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.Lock()
	podName, ok := e.sessions[id]
	msgCnt := e.sessionMsgCnt[id]
	e.sessionMsgCnt[id] = msgCnt + 1
	e.mu.Unlock()

	if !ok {
		podName = fmt.Sprintf("abox-%s", id)
	}

	args := e.kubectlArgs("exec", podName, "-c", "agent", "--")
	args = append(args,
		"env", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
		"PATH=/home/agent/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)
	args = append(args, "claude", "-p", "--dangerously-skip-permissions",
		"--output-format", "stream-json", "--verbose")
	if msgCnt > 0 {
		args = append(args, "--continue")
	}
	args = append(args, message)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if e.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+e.kubeconfig)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	e.logger.Info("streaming message to session", "pod", podName)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("kubectl exec start: %w", err)
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type  string `json:"type"`
			Event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
			Result string `json:"result"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "stream_event":
			if event.Event.Type == "content_block_delta" && event.Event.Delta.Type == "text_delta" {
				token := event.Event.Delta.Text
				fullResponse.WriteString(token)
				if onToken != nil {
					onToken(token)
				}
			}
		case "result":
			if event.Result != "" && fullResponse.Len() == 0 {
				fullResponse.WriteString(event.Result)
			}
		}
	}

	cmd.Wait()
	return fullResponse.String(), nil
}

// StopSession deletes the session pod.
func (e *k8sExecutor) StopSession(ctx context.Context, id string) error {
	podName := fmt.Sprintf("abox-%s", id)

	e.logger.Info("stopping session pod", "name", podName)

	gracePeriod := int64(10)
	err := e.client.CoreV1().Pods(e.namespace).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})

	e.mu.Lock()
	delete(e.sessions, id)
	delete(e.sessionMsgCnt, id)
	e.mu.Unlock()

	return err
}

// kubectlArgs builds kubectl arguments with namespace and optional kubeconfig.
func (e *k8sExecutor) kubectlArgs(args ...string) []string {
	var result []string
	if e.kubeconfig != "" {
		result = append(result, "--kubeconfig", e.kubeconfig)
	}
	result = append(result, "-n", e.namespace)
	result = append(result, args...)
	return result
}

// RecoverSessions lists running session pods and re-registers them.
func (e *k8sExecutor) RecoverSessions(ctx context.Context) ([]string, error) {
	pods, err := e.client.CoreV1().Pods(e.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=agentbox,agentbox/mode=session",
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	var ids []string
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		runID := pod.Labels["agentbox/run"]
		if runID == "" {
			continue
		}
		e.mu.Lock()
		e.sessions[runID] = pod.Name
		e.mu.Unlock()
		ids = append(ids, runID)
	}
	return ids, nil
}

// UploadFile copies a file into a running session pod at /workspace/uploads/.
func (e *k8sExecutor) UploadFile(ctx context.Context, id string, filename string, data []byte) error {
	e.mu.Lock()
	podName, ok := e.sessions[id]
	e.mu.Unlock()

	if !ok {
		podName = fmt.Sprintf("abox-%s", id)
	}

	// Ensure uploads directory exists, then write file via kubectl exec + base64
	encoded := base64Encode(data)
	script := fmt.Sprintf("mkdir -p /workspace/uploads && echo '%s' | base64 -d > /workspace/uploads/%s", encoded, filename)
	args := e.kubectlArgs("exec", podName, "-c", "agent", "--", "sh", "-c", script)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if e.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+e.kubeconfig)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kubectl exec: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// StreamLogs streams pod logs line by line via a channel.
func (e *k8sExecutor) StreamLogs(ctx context.Context, id string) (<-chan string, error) {
	e.mu.Lock()
	podName, ok := e.sessions[id]
	e.mu.Unlock()

	if !ok {
		podName = fmt.Sprintf("abox-%s", id)
	}

	args := e.kubectlArgs("logs", "-f", "--tail=100", podName, "-c", "agent")
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if e.kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+e.kubeconfig)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("kubectl logs start: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text():
			}
		}
	}()

	return ch, nil
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
