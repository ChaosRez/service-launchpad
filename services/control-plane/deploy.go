package main

// adds a small `kubectl apply -f -` deployer abstraction

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const defaultKubectlBinary = "kubectl"

type applyResult struct {
	Command string `json:"command"`
	Output  string `json:"output,omitempty"`
}

type manifestDeployer interface {
	Apply(ctx context.Context, manifestsYAML string) (applyResult, error)
}

type kubectlDeployer struct {
	binary  string
	context string
}

func newKubectlDeployer(binary, kubectlContext string) manifestDeployer {
	if strings.TrimSpace(binary) == "" {
		binary = defaultKubectlBinary
	}

	return &kubectlDeployer{
		binary:  binary,
		context: strings.TrimSpace(kubectlContext),
	}
}

func (d *kubectlDeployer) Apply(ctx context.Context, manifestsYAML string) (applyResult, error) {
	args := make([]string, 0, 5)
	if d.context != "" {
		args = append(args, "--context", d.context)
	}
	args = append(args, "apply", "-f", "-")

	cmd := exec.CommandContext(ctx, d.binary, args...)
	cmd.Stdin = strings.NewReader(manifestsYAML)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		if output == "" {
			output = err.Error()
		}

		return applyResult{
			Command: buildCommandString(d.binary, args),
			Output:  output,
		}, fmt.Errorf("%s: %w", output, err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String())
	}

	return applyResult{
		Command: buildCommandString(d.binary, args),
		Output:  output,
	}, nil
}

func buildCommandString(binary string, args []string) string {
	parts := append([]string{binary}, args...)
	return strings.Join(parts, " ")
}
