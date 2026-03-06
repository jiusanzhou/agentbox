package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/x"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Config for kubernetes executor.
type Config struct {
	Kubeconfig string `json:"kubeconfig,omitempty" yaml:"kubeconfig"`
	Namespace  string `json:"namespace" yaml:"namespace"`
	Image      string `json:"image" yaml:"image"`
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

	return &k8sExecutor{
		client:    client,
		namespace: ns,
		image:     cfg.Image,
		logger:    slog.Default(),
	}, nil
}

func (e *k8sExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	jobName := fmt.Sprintf("agentbox-%s", req.ID)

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

func (e *k8sExecutor) Logs(ctx context.Context, id string) (string, error) {
	// TODO: get pod logs by job label selector
	return "", nil
}

func (e *k8sExecutor) Stop(ctx context.Context, id string) error {
	jobName := fmt.Sprintf("agentbox-%s", id)
	propagation := metav1.DeletePropagationForeground
	return e.client.BatchV1().Jobs(e.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}

func (e *k8sExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	return "", fmt.Errorf("session mode not supported by k8s executor")
}

func (e *k8sExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	return "", fmt.Errorf("session mode not supported by k8s executor")
}

func (e *k8sExecutor) StopSession(ctx context.Context, id string) error {
	return fmt.Errorf("session mode not supported by k8s executor")
}
