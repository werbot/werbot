//go:build cluster || load || redteam

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// standDir is test/cluster, found from this source file so test/load can drive the stand too.
var standDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

// Project is the compose project: COMPOSE_PROJECT_NAME (`make e2e-cluster` sets it) or werbot-cluster.
func Project() string {
	if p := os.Getenv("COMPOSE_PROJECT_NAME"); p != "" {
		return p
	}
	return "werbot-cluster"
}

// Helpers that take a testing.TB use context.Background(), not t.Context():
// scenarios call them from t.Cleanup, after t.Context() is canceled.

// Compose runs docker compose on the stand with the Up interpolation environment.
func (s *Stand) Compose(ctx context.Context, args ...string) (string, error) {
	return s.ComposeEnv(ctx, nil, args...)
}

// ComposeEnv is Compose with env overriding the Up interpolation environment.
func (s *Stand) ComposeEnv(ctx context.Context, env map[string]string, args ...string) (string, error) {
	full := append([]string{"compose", "-p", s.project, "-f", filepath.Join(standDir, "docker-compose.yaml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...) //nolint:gosec // fixed binary, arguments from the harness
	cmd.Env = os.Environ()
	for _, m := range []map[string]string{s.env, env} {
		for k, v := range m {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker compose %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// Addr is the published host address of a container port: "127.0.0.1:N".
// Host ports change when a container restarts; resolve them again after each restart.
func (s *Stand) Addr(t testing.TB, service string, port int) string {
	t.Helper()
	addr, err := s.port(context.Background(), service, port)
	require.NoError(t, err)
	return addr
}

func (s *Stand) port(ctx context.Context, service string, port int) (string, error) {
	out, err := s.Compose(ctx, "port", service, fmt.Sprint(port))
	if err != nil {
		return "", err
	}
	addr := strings.TrimSpace(out)
	if addr == "" || strings.HasSuffix(addr, ":0") {
		return "", fmt.Errorf("%s:%d is not published", service, port)
	}
	return addr, nil
}

// Container is the docker inspect state of a service container.
type Container struct {
	PID       int
	Running   bool
	StartedAt time.Time
	Networks  []string // sorted network names
}

// Container inspects the container of service, stopped or running.
func (s *Stand) Container(t testing.TB, service string) Container {
	t.Helper()
	c, err := s.inspect(context.Background(), service)
	require.NoError(t, err)
	return c
}

func (s *Stand) inspect(ctx context.Context, service string) (Container, error) {
	id, err := s.Compose(ctx, "ps", "-a", "-q", service)
	if err != nil {
		return Container{}, err
	}
	if id = strings.TrimSpace(id); id == "" {
		return Container{}, fmt.Errorf("no container for %s", service)
	}
	out, err := exec.CommandContext(ctx, "docker", "inspect", id).Output() //nolint:gosec // fixed binary, container id from compose
	if err != nil {
		return Container{}, fmt.Errorf("docker inspect %s: %w", service, err)
	}
	var v []struct {
		State struct {
			Pid       int
			Running   bool
			StartedAt time.Time
		}
		NetworkSettings struct {
			Networks map[string]struct{}
		}
	}
	if err := json.Unmarshal(out, &v); err != nil || len(v) != 1 {
		return Container{}, fmt.Errorf("docker inspect %s: %d entries, %v", service, len(v), err)
	}
	c := Container{PID: v[0].State.Pid, Running: v[0].State.Running, StartedAt: v[0].State.StartedAt}
	for name := range v[0].NetworkSettings.Networks {
		c.Networks = append(c.Networks, name)
	}
	slices.Sort(c.Networks)
	return c, nil
}

// buildTargets builds the static echo target (ruling C5) and SSH target
// (stage 16 ruling G3) for the docker server's architecture into .bin.
func buildTargets(ctx context.Context) error {
	arch, err := exec.CommandContext(ctx, "docker", "version", "-f", "{{.Server.Arch}}").Output()
	if err != nil {
		return fmt.Errorf("docker version: %w", err)
	}
	env := []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=" + strings.TrimSpace(string(arch))}
	for _, name := range []string{"echotarget", "sshtarget"} {
		if err := goBuild(ctx, env, filepath.Join(standDir, ".bin", name), "./internal/e2e/"+name); err != nil {
			return err
		}
	}
	return nil
}

// goBuild builds pkg of this module into out with env added to the environment.
func goBuild(ctx context.Context, env []string, out, pkg string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, pkg) //nolint:gosec // fixed binary, package and output from the harness
	cmd.Dir = filepath.Join(standDir, "..", "..")
	cmd.Env = append(os.Environ(), env...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w\n%s", pkg, err, b)
	}
	return nil
}
