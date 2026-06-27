package apiserver

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
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

type KubectlPodExecutor struct {
	binary string
}

func NewKubectlPodExecutor(binary string) *KubectlPodExecutor {
	if binary == "" {
		binary = "kubectl"
	}
	return &KubectlPodExecutor{binary: binary}
}

func (e *KubectlPodExecutor) Exec(ctx context.Context, namespace string, podName string, containerName string, req ExecRequest) (ExecResponse, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return ExecResponse{}, errors.New("command is required")
	}
	if e == nil || e.binary == "" {
		return ExecResponse{}, errors.New("pod executor is not configured")
	}

	command := append([]string(nil), req.Command...)
	if req.Workdir != "" {
		command = append([]string{"/bin/sh", "-lc", "cd " + shellQuote(req.Workdir) + " && exec \"$@\"", "--"}, command...)
	}
	if len(req.Env) > 0 {
		envCommand := make([]string, 0, len(req.Env)*2+len(command)+1)
		envCommand = append(envCommand, "env")
		for key, value := range req.Env {
			envCommand = append(envCommand, key+"="+value)
		}
		envCommand = append(envCommand, command...)
		command = envCommand
	}

	args := []string{"exec", "-n", namespace, podName, "-c", containerName, "--"}
	args = append(args, command...)

	started := time.Now().UTC()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, e.binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}
	err := cmd.Run()
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		response.ExitCode = exitErr.ExitCode()
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
