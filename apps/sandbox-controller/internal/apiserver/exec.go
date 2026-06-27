package apiserver

import (
	"bytes"
	"context"
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	kexec "k8s.io/client-go/util/exec"
)

type ExecRequest struct {
	Command        []string          `json:"command"`
	Stdin          string            `json:"stdin,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Workdir        string            `json:"workdir,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds,omitempty"`
}

type ExecResponse struct {
	ExitCode   int       `json:"exitCode"`
	Stdout     string    `json:"stdout,omitempty"`
	Stderr     string    `json:"stderr,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

type PodExecutor interface {
	Exec(ctx context.Context, namespace string, podName string, containerName string, req ExecRequest) (ExecResponse, error)
}

type KubernetesPodExecutor struct {
	config    *rest.Config
	clientset kubernetes.Interface
}

func NewKubernetesPodExecutor(config *rest.Config, clientset kubernetes.Interface) *KubernetesPodExecutor {
	return &KubernetesPodExecutor{config: config, clientset: clientset}
}

func (e *KubernetesPodExecutor) Exec(ctx context.Context, namespace string, podName string, containerName string, req ExecRequest) (ExecResponse, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return ExecResponse{}, errors.New("command is required")
	}
	if e == nil || e.config == nil || e.clientset == nil {
		return ExecResponse{}, errors.New("pod executor is not configured")
	}

	command := append([]string(nil), req.Command...)
	if req.Workdir != "" {
		command = append([]string{"/bin/sh", "-lc", "cd " + shellQuote(req.Workdir) + " && exec \"$@\"", "--"}, command...)
	}
	if len(req.Env) > 0 {
		envCommand := make([]string, 0, len(req.Env)*2+len(command)+2)
		envCommand = append(envCommand, "env")
		for key, value := range req.Env {
			envCommand = append(envCommand, key+"="+value)
		}
		envCommand = append(envCommand, command...)
		command = envCommand
	}

	started := time.Now().UTC()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	execReq := e.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     req.Stdin != "",
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, clientgoscheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(e.config, "POST", execReq.URL())
	if err != nil {
		return ExecResponse{}, err
	}
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  bytes.NewBufferString(req.Stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	finished := time.Now().UTC()

	response := ExecResponse{
		ExitCode:   0,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		StartedAt:  started,
		FinishedAt: finished,
	}
	if err == nil {
		return response, nil
	}
	var exitErr kexec.ExitError
	if errors.As(err, &exitErr) {
		response.ExitCode = exitErr.ExitStatus()
		return response, nil
	}
	return response, err
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	var out bytes.Buffer
	out.WriteByte('\'')
	for _, r := range value {
		if r == '\'' {
			out.WriteString("'\\''")
			continue
		}
		out.WriteRune(r)
	}
	out.WriteByte('\'')
	return out.String()
}
