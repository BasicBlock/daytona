// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package gvisor

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/daytonaio/runner/pkg/api/dto"
	"github.com/daytonaio/runner/pkg/common"
	"github.com/daytonaio/runner/pkg/models/enums"
	"github.com/daytonaio/runner/pkg/snapshotbundle"
	"github.com/daytonaio/runner/pkg/storage"
	"github.com/docker/docker/api/types/container"
	dockernetwork "github.com/docker/docker/api/types/network"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	defaultDaemonPort = "2280"
	defaultSSHPort    = "22220"
	runscNetworkName  = "daytona-runsc"
)

type RuntimeConfig struct {
	StateDir                  string
	BridgeName                string
	BridgeCIDR                string
	DaemonPath                string
	ComputerUsePluginPath     string
	ResourceLimitsDisabled    bool
	DaemonStartTimeoutSec     int
	SandboxStartTimeoutSec    int
	UseSnapshotEntrypoint     bool
	InitializeDaemonTelemetry bool
	Logger                    *slog.Logger
}

type Runtime struct {
	client                    *Client
	stateDir                  string
	sandboxesDir              string
	imagesDir                 string
	bridgeName                string
	bridgeCIDR                *net.IPNet
	bridgeIP                  net.IP
	daemonPath                string
	computerUsePluginPath     string
	resourceLimitsDisabled    bool
	daemonStartTimeoutSec     int
	sandboxStartTimeoutSec    int
	useSnapshotEntrypoint     bool
	initializeDaemonTelemetry bool
	logger                    *slog.Logger
}

type SandboxRuntimeState struct {
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Snapshot        string            `json:"snapshot"`
	BaseImageRef    string            `json:"baseImageRef"`
	BaseImageDigest string            `json:"baseImageDigest,omitempty"`
	RootfsDir       string            `json:"rootfsDir"`
	BundleDir       string            `json:"bundleDir"`
	NetNSName       string            `json:"netnsName"`
	NetNSPath       string            `json:"netnsPath"`
	HostVeth        string            `json:"hostVeth"`
	IP              string            `json:"ip"`
	AuthToken       string            `json:"authToken,omitempty"`
	Entrypoint      []string          `json:"entrypoint,omitempty"`
	Cmd             []string          `json:"cmd,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	WorkDir         string            `json:"workDir,omitempty"`
	OsUser          string            `json:"osUser,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	Status          string            `json:"status"`
}

type imageData struct {
	Config *v1.ConfigFile
	Digest string
}

func NewRuntime(ctx context.Context, cfg RuntimeConfig) (*Runtime, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("raw runsc lifecycle requires Linux")
	}
	if cfg.StateDir == "" {
		return nil, fmt.Errorf("runsc state dir is required")
	}
	if cfg.BridgeName == "" {
		cfg.BridgeName = "daytona0"
	}
	if cfg.BridgeCIDR == "" {
		cfg.BridgeCIDR = "172.29.0.1/16"
	}
	if cfg.DaemonPath == "" {
		return nil, fmt.Errorf("daemon path is required")
	}
	if cfg.DaemonStartTimeoutSec <= 0 {
		cfg.DaemonStartTimeoutSec = 60
	}
	if cfg.SandboxStartTimeoutSec <= 0 {
		cfg.SandboxStartTimeoutSec = 30
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	bridgeIP, bridgeCIDR, err := net.ParseCIDR(cfg.BridgeCIDR)
	if err != nil {
		return nil, fmt.Errorf("parse RUNSC_BRIDGE_CIDR: %w", err)
	}
	bridgeCIDR.IP = bridgeIP

	client, err := NewClientFromConfig(cfg.Logger)
	if err != nil {
		return nil, err
	}

	rt := &Runtime{
		client:                    client,
		stateDir:                  cfg.StateDir,
		sandboxesDir:              filepath.Join(cfg.StateDir, "sandboxes"),
		imagesDir:                 filepath.Join(cfg.StateDir, "images"),
		bridgeName:                cfg.BridgeName,
		bridgeCIDR:                bridgeCIDR,
		bridgeIP:                  bridgeIP,
		daemonPath:                cfg.DaemonPath,
		computerUsePluginPath:     cfg.ComputerUsePluginPath,
		resourceLimitsDisabled:    cfg.ResourceLimitsDisabled,
		daemonStartTimeoutSec:     cfg.DaemonStartTimeoutSec,
		sandboxStartTimeoutSec:    cfg.SandboxStartTimeoutSec,
		useSnapshotEntrypoint:     cfg.UseSnapshotEntrypoint,
		initializeDaemonTelemetry: cfg.InitializeDaemonTelemetry,
		logger:                    cfg.Logger.With(slog.String("component", "runsc-runtime")),
	}

	if err := os.MkdirAll(rt.sandboxesDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rt.imagesDir, 0755); err != nil {
		return nil, err
	}
	if err := rt.ensureBridge(ctx); err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *Runtime) Exists(id string) bool {
	_, err := os.Stat(r.statePath(id))
	return err == nil
}

func (r *Runtime) Create(ctx context.Context, sandbox dto.CreateSandboxDTO) (string, string, error) {
	if sandbox.GpuQuota > 0 {
		return "", "", fmt.Errorf("raw runsc lifecycle does not support GPU sandboxes yet")
	}
	if sandbox.Id == "" {
		return "", "", fmt.Errorf("sandbox id is required")
	}

	if r.Exists(sandbox.Id) {
		return r.Start(ctx, sandbox.Id, sandbox.AuthToken, sandbox.Metadata)
	}

	if err := os.MkdirAll(r.sandboxDir(sandbox.Id), 0755); err != nil {
		return "", "", err
	}

	var (
		img    *imageData
		bundle *snapshotbundle.CachedBundle
		err    error
	)
	baseImageRef := sandbox.Snapshot
	baseImageDigest := ""
	config := snapshotbundle.RuntimeConfig{}
	labels := map[string]string(nil)
	if storage.IsSnapshotStoreRef(sandbox.Snapshot) {
		store, err := storage.GetSnapshotStoreClient(ctx)
		if err != nil {
			return "", "", err
		}
		bundle, err = snapshotbundle.CacheFromRef(ctx, store, sandbox.Snapshot)
		if err != nil {
			return "", "", err
		}
		baseImageRef = bundle.Manifest.BaseImageRef
		baseImageDigest = bundle.Manifest.BaseImageDigest
		config = bundle.Manifest.Config
		labels = bundle.Manifest.Labels
	}

	rootfsDir := filepath.Join(r.sandboxDir(sandbox.Id), "rootfs")
	bundleDir := filepath.Join(r.sandboxDir(sandbox.Id), "bundle")
	if err := os.RemoveAll(rootfsDir); err != nil {
		return "", "", err
	}
	if err := os.RemoveAll(bundleDir); err != nil {
		return "", "", err
	}

	img, err = r.unpackImage(ctx, baseImageRef, sandbox.Registry, rootfsDir)
	if err != nil {
		return "", "", err
	}
	if baseImageDigest == "" {
		baseImageDigest = img.Digest
	}

	state := &SandboxRuntimeState{
		ID:              sandbox.Id,
		Name:            sandbox.Name,
		Snapshot:        sandbox.Snapshot,
		BaseImageRef:    baseImageRef,
		BaseImageDigest: baseImageDigest,
		RootfsDir:       rootfsDir,
		BundleDir:       bundleDir,
		AuthToken:       deref(sandbox.AuthToken),
		Env:             mergeEnv(envSliceToMap(img.Config.Config.Env), config.Env, sandboxEnv(sandbox), sandbox.Env),
		WorkDir:         firstNonEmpty(config.WorkDir, img.Config.Config.WorkingDir),
		OsUser:          firstNonEmpty(config.OsUser, sandbox.OsUser, img.Config.Config.User),
		Labels:          mergeLabels(img.Config.Config.Labels, labels, sandbox.Name, deref(sandbox.AuthToken)),
		CreatedAt:       time.Now().UTC(),
		Status:          string(enums.SandboxStateStopped),
	}
	state.Entrypoint, state.Cmd = r.processArgs(sandbox, img.Config, config)

	if err := r.setupNetwork(ctx, state); err != nil {
		return "", "", err
	}
	if err := r.prepareRootfs(state); err != nil {
		return "", "", err
	}
	if err := r.writeBundle(state, sandbox); err != nil {
		return "", "", err
	}
	if err := r.saveState(state); err != nil {
		return "", "", err
	}

	if sandbox.SkipStart != nil && *sandbox.SkipStart {
		return sandbox.Id, "", nil
	}

	if bundle != nil {
		if err := r.client.Restore(ctx, sandbox.Id, bundleDir, bundle.CheckpointDir, bundle.FilesystemDir); err != nil {
			return "", "", err
		}
	} else if err := r.client.Run(ctx, sandbox.Id, bundleDir); err != nil {
		return "", "", err
	}

	state.Status = string(enums.SandboxStateStarted)
	if err := r.saveState(state); err != nil {
		return "", "", err
	}

	daemonVersion, err := r.waitForDaemonRunning(ctx, state, sandbox.AuthToken)
	if err != nil {
		return "", "", err
	}
	return sandbox.Id, daemonVersion, nil
}

func (r *Runtime) Start(ctx context.Context, sandboxID string, authToken *string, _ map[string]string) (string, string, error) {
	state, err := r.loadState(sandboxID)
	if err != nil {
		return "", "", err
	}

	if running, _ := r.isRunning(ctx, sandboxID); !running {
		if err := r.ensureBridge(ctx); err != nil {
			return "", "", err
		}
		if err := r.setupNetwork(ctx, state); err != nil {
			return "", "", err
		}
		if err := r.client.Run(ctx, sandboxID, state.BundleDir); err != nil {
			return "", "", err
		}
	}

	if authToken != nil && *authToken != "" {
		state.AuthToken = *authToken
	}
	state.Status = string(enums.SandboxStateStarted)
	if err := r.saveState(state); err != nil {
		return "", "", err
	}

	daemonVersion, err := r.waitForDaemonRunning(ctx, state, authToken)
	if err != nil {
		return "", "", err
	}
	return sandboxID, daemonVersion, nil
}

func (r *Runtime) Stop(ctx context.Context, sandboxID string, force bool) error {
	state, err := r.loadState(sandboxID)
	if err != nil {
		return err
	}

	sig := "TERM"
	if force {
		sig = "KILL"
	}
	if err := r.client.Kill(ctx, sandboxID, sig); err != nil {
		r.logger.WarnContext(ctx, "runsc kill failed", "sandboxId", sandboxID, "error", err)
	}
	if err := r.client.Delete(ctx, sandboxID); err != nil {
		r.logger.WarnContext(ctx, "runsc delete after stop failed", "sandboxId", sandboxID, "error", err)
	}
	state.Status = string(enums.SandboxStateStopped)
	return r.saveState(state)
}

func (r *Runtime) Destroy(ctx context.Context, sandboxID string) error {
	state, err := r.loadState(sandboxID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = r.client.Kill(ctx, sandboxID, "KILL")
	_ = r.client.Delete(ctx, sandboxID)
	if state != nil {
		r.teardownNetwork(ctx, state)
	}
	return os.RemoveAll(r.sandboxDir(sandboxID))
}

func (r *Runtime) GetSandboxState(ctx context.Context, sandboxID string) (enums.SandboxState, error) {
	state, err := r.loadState(sandboxID)
	if err != nil {
		if os.IsNotExist(err) {
			return enums.SandboxStateDestroyed, nil
		}
		return enums.SandboxStateError, err
	}
	running, err := r.isRunning(ctx, sandboxID)
	if err == nil && running {
		return enums.SandboxStateStarted, nil
	}
	switch state.Status {
	case string(enums.SandboxStateStopped):
		return enums.SandboxStateStopped, nil
	case string(enums.SandboxStateStarted):
		return enums.SandboxStateStopped, nil
	default:
		return enums.SandboxState(state.Status), nil
	}
}

func (r *Runtime) Inspect(ctx context.Context, sandboxID string) (*container.InspectResponse, error) {
	state, err := r.loadState(sandboxID)
	if err != nil {
		return nil, err
	}
	running, _ := r.isRunning(ctx, sandboxID)
	status := "exited"
	if running {
		status = "running"
	}
	return &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:    state.ID,
			Name:  "/" + state.ID,
			Image: state.BaseImageRef,
			State: &container.State{
				Running: running,
				Status:  container.ContainerState(status),
			},
		},
		Config: &container.Config{
			Hostname:   state.ID,
			Image:      state.BaseImageRef,
			WorkingDir: state.WorkDir,
			Env:        envMapToSlice(state.Env),
			Entrypoint: state.Entrypoint,
			Cmd:        state.Cmd,
			Labels:     state.Labels,
		},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*dockernetwork.EndpointSettings{
				runscNetworkName: {
					IPAddress: state.IP,
				},
			},
		},
	}, nil
}

func (r *Runtime) CreateSnapshotFromSandbox(ctx context.Context, sandboxID string, name string, store storage.SnapshotStoreClient) (*dto.SnapshotInfoResponse, error) {
	state, err := r.loadState(sandboxID)
	if err != nil {
		return nil, err
	}
	manifest, err := r.client.CreateSnapshot(ctx, store, SnapshotOptions{
		SandboxID:       sandboxID,
		Name:            name,
		BaseImageRef:    state.BaseImageRef,
		BaseImageDigest: state.BaseImageDigest,
		RuntimeConfig: snapshotbundle.RuntimeConfig{
			Entrypoint: state.Entrypoint,
			Cmd:        state.Cmd,
			Env:        state.Env,
			WorkDir:    state.WorkDir,
			OsUser:     state.OsUser,
		},
		Labels: state.Labels,
	})
	if err != nil {
		return nil, err
	}
	return &dto.SnapshotInfoResponse{
		Name:       manifest.Ref,
		SizeGB:     float64(manifest.SizeBytes) / (1024 * 1024 * 1024),
		Entrypoint: manifest.Config.Entrypoint,
		Cmd:        manifest.Config.Cmd,
		Hash:       manifest.Hash,
	}, nil
}

func (r *Runtime) Fork(ctx context.Context, sourceSandboxID string, newSandboxID string, targetAuthToken string) (string, error) {
	source, err := r.loadState(sourceSandboxID)
	if err != nil {
		return "", err
	}
	if _, err := r.loadState(newSandboxID); err == nil {
		forked, err := r.loadState(newSandboxID)
		if err != nil {
			return "", err
		}
		return r.waitForDaemonRunning(ctx, forked, &targetAuthToken)
	}

	forkDir := filepath.Join(r.sandboxDir(newSandboxID), "fork")
	checkpointDir := filepath.Join(forkDir, "checkpoint")
	filesystemDir := filepath.Join(forkDir, "filesystem")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filesystemDir, 0755); err != nil {
		return "", err
	}

	paused := false
	if err := r.client.Pause(ctx, sourceSandboxID); err != nil {
		return "", fmt.Errorf("pause source sandbox before fork: %w", err)
	}
	paused = true
	defer func() {
		if paused {
			if err := r.client.Resume(context.Background(), sourceSandboxID); err != nil {
				r.logger.ErrorContext(context.Background(), "Failed to resume source sandbox after fork checkpoint", "sandboxId", sourceSandboxID, "error", err)
			}
		}
	}()
	if err := r.client.FSCheckpoint(ctx, sourceSandboxID, filesystemDir); err != nil {
		return "", fmt.Errorf("create fork filesystem checkpoint: %w", err)
	}
	if err := r.client.Checkpoint(ctx, sourceSandboxID, checkpointDir); err != nil {
		return "", fmt.Errorf("create fork memory checkpoint: %w", err)
	}
	if err := r.client.Resume(ctx, sourceSandboxID); err != nil {
		return "", fmt.Errorf("resume source sandbox after fork checkpoint: %w", err)
	}
	paused = false

	rootfsDir := filepath.Join(r.sandboxDir(newSandboxID), "rootfs")
	bundleDir := filepath.Join(r.sandboxDir(newSandboxID), "bundle")
	if err := os.RemoveAll(rootfsDir); err != nil {
		return "", err
	}
	if err := os.RemoveAll(bundleDir); err != nil {
		return "", err
	}
	if _, err := r.unpackImage(ctx, source.BaseImageRef, nil, rootfsDir); err != nil {
		return "", err
	}

	forked := *source
	forked.ID = newSandboxID
	forked.Name = ""
	forked.RootfsDir = rootfsDir
	forked.BundleDir = bundleDir
	forked.AuthToken = targetAuthToken
	forked.CreatedAt = time.Now().UTC()
	forked.Status = string(enums.SandboxStateStopped)
	if forked.Labels == nil {
		forked.Labels = map[string]string{}
	}
	forked.Labels["daytona.auth_token"] = targetAuthToken

	if err := r.setupNetwork(ctx, &forked); err != nil {
		return "", err
	}
	if err := r.prepareRootfs(&forked); err != nil {
		return "", err
	}
	if err := r.writeBundle(&forked, dto.CreateSandboxDTO{
		Id:           newSandboxID,
		Snapshot:     source.Snapshot,
		OsUser:       source.OsUser,
		CpuQuota:     1,
		MemoryQuota:  1,
		StorageQuota: 1,
	}); err != nil {
		return "", err
	}
	if err := r.saveState(&forked); err != nil {
		return "", err
	}
	if err := r.client.Restore(ctx, newSandboxID, bundleDir, checkpointDir, filesystemDir); err != nil {
		return "", err
	}
	forked.Status = string(enums.SandboxStateStarted)
	if err := r.saveState(&forked); err != nil {
		return "", err
	}
	return r.waitForDaemonRunning(ctx, &forked, &targetAuthToken)
}

func (r *Runtime) processArgs(sandbox dto.CreateSandboxDTO, img *v1.ConfigFile, snapshotConfig snapshotbundle.RuntimeConfig) ([]string, []string) {
	if r.useSnapshotEntrypoint {
		entrypoint := snapshotConfig.Entrypoint
		if len(entrypoint) == 0 {
			entrypoint = sandbox.Entrypoint
		}
		if len(entrypoint) == 0 {
			entrypoint = img.Config.Entrypoint
		}
		cmd := snapshotConfig.Cmd
		if len(cmd) == 0 {
			cmd = img.Config.Cmd
		}
		return entrypoint, cmd
	}

	cmd := []string{}
	if len(sandbox.Entrypoint) != 0 {
		cmd = append(cmd, sandbox.Entrypoint...)
	} else if !slices.Equal(img.Config.Entrypoint, []string{common.DAEMON_PATH}) {
		cmd = append(cmd, img.Config.Entrypoint...)
	}
	if len(cmd) == 0 {
		cmd = append(cmd, img.Config.Cmd...)
	}
	return []string{common.DAEMON_PATH}, cmd
}

func (r *Runtime) prepareRootfs(state *SandboxRuntimeState) error {
	if err := os.MkdirAll(filepath.Join(state.RootfsDir, "usr/local/bin"), 0755); err != nil {
		return err
	}
	if err := ensureFile(filepath.Join(state.RootfsDir, strings.TrimPrefix(common.DAEMON_PATH, "/")), 0755); err != nil {
		return err
	}
	if r.computerUsePluginPath != "" {
		pluginInfo, err := os.Stat(r.computerUsePluginPath)
		if err != nil {
			return fmt.Errorf("stat computer-use plugin: %w", err)
		}
		pluginTarget := filepath.Join(state.RootfsDir, "usr/local/lib/daytona-computer-use")
		if pluginInfo.IsDir() {
			if err := os.MkdirAll(pluginTarget, 0755); err != nil {
				return err
			}
		} else if err := ensureFile(pluginTarget, 0755); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(state.RootfsDir, "etc"), 0755); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(state.RootfsDir, "etc", "hostname"), []byte(state.ID+"\n"), 0644)
	_ = os.WriteFile(filepath.Join(state.RootfsDir, "etc", "hosts"), []byte(fmt.Sprintf("127.0.0.1 localhost\n%s %s\n", state.IP, state.ID)), 0644)
	if resolv, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		_ = os.WriteFile(filepath.Join(state.RootfsDir, "etc", "resolv.conf"), resolv, 0644)
	}
	return nil
}

func (r *Runtime) writeBundle(state *SandboxRuntimeState, sandbox dto.CreateSandboxDTO) error {
	if err := os.MkdirAll(state.BundleDir, 0755); err != nil {
		return err
	}
	spec := r.ociSpec(state, sandbox)
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(state.BundleDir, "config.json"), data, 0644)
}

func (r *Runtime) ociSpec(state *SandboxRuntimeState, sandbox dto.CreateSandboxDTO) specs.Spec {
	args := append([]string{}, state.Entrypoint...)
	args = append(args, state.Cmd...)
	if len(args) == 0 {
		args = []string{common.DAEMON_PATH}
	}
	caps := []string{
		"CAP_CHOWN", "CAP_DAC_OVERRIDE", "CAP_FOWNER", "CAP_FSETID",
		"CAP_KILL", "CAP_SETGID", "CAP_SETUID", "CAP_SETPCAP",
		"CAP_NET_BIND_SERVICE", "CAP_NET_RAW", "CAP_SYS_CHROOT",
		"CAP_MKNOD", "CAP_AUDIT_WRITE", "CAP_SETFCAP",
	}
	env := envMapToSlice(state.Env)
	if state.WorkDir == "" {
		env = append(env, "DAYTONA_USER_HOME_AS_WORKDIR=true")
	}

	mounts := []specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "strictatime", "mode=755", "size=65536k"}},
		{Destination: "/dev/pts", Type: "devpts", Source: "devpts", Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		{Destination: "/dev/shm", Type: "tmpfs", Source: "shm", Options: []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"}},
		{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"mode=1777"}},
		{Destination: common.DAEMON_PATH, Type: "bind", Source: r.daemonPath, Options: []string{"rbind", "ro"}},
	}
	if r.computerUsePluginPath != "" {
		mounts = append(mounts, specs.Mount{
			Destination: "/usr/local/lib/daytona-computer-use",
			Type:        "bind",
			Source:      r.computerUsePluginPath,
			Options:     []string{"rbind", "ro"},
		})
	}

	linux := &specs.Linux{
		Namespaces: []specs.LinuxNamespace{
			{Type: specs.PIDNamespace},
			{Type: specs.IPCNamespace},
			{Type: specs.UTSNamespace},
			{Type: specs.MountNamespace},
			{Type: specs.NetworkNamespace, Path: state.NetNSPath},
		},
		Devices: defaultLinuxDevices(),
		MaskedPaths: []string{
			"/proc/acpi", "/proc/asound", "/proc/kcore", "/proc/keys",
			"/proc/latency_stats", "/proc/timer_list", "/proc/timer_stats",
			"/proc/sched_debug", "/sys/firmware", "/proc/scsi",
		},
		ReadonlyPaths: []string{"/proc/bus", "/proc/fs", "/proc/irq", "/proc/sys", "/proc/sysrq-trigger"},
	}
	if !r.resourceLimitsDisabled {
		linux.Resources = &specs.LinuxResources{
			CPU: &specs.LinuxCPU{
				Period: uint64Ptr(100000),
				Quota:  int64Ptr(maxInt64(sandbox.CpuQuota, 1) * 100000),
			},
			Memory: &specs.LinuxMemory{
				Limit: int64Ptr(common.GBToBytes(float64(maxInt64(sandbox.MemoryQuota, 1)))),
			},
		}
	}

	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Terminal: false,
			User:     specs.User{UID: 0, GID: 0},
			Args:     args,
			Env:      env,
			Cwd:      firstNonEmpty(state.WorkDir, "/"),
			Capabilities: &specs.LinuxCapabilities{
				Bounding:    caps,
				Effective:   caps,
				Inheritable: caps,
				Permitted:   caps,
				Ambient:     caps,
			},
			NoNewPrivileges: false,
		},
		Root: &specs.Root{
			Path:     state.RootfsDir,
			Readonly: false,
		},
		Hostname: state.ID,
		Mounts:   mounts,
		Linux:    linux,
	}
}

func (r *Runtime) unpackImage(ctx context.Context, imageRef string, registry *dto.RegistryDTO, destRootfs string) (*imageData, error) {
	if imageRef == "" {
		return nil, fmt.Errorf("image ref is required")
	}
	if err := os.MkdirAll(destRootfs, 0755); err != nil {
		return nil, err
	}

	ref, err := parseImageRef(imageRef)
	if err != nil {
		return nil, err
	}
	opts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithPlatform(v1.Platform{OS: "linux", Architecture: runtime.GOARCH}),
	}
	if registry != nil && registry.HasAuth() {
		opts = append(opts, remote.WithAuth(&authn.Basic{Username: *registry.Username, Password: *registry.Password}))
	} else {
		opts = append(opts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	img, err := remote.Image(ref, opts...)
	if err != nil {
		return nil, fmt.Errorf("pull image %s for runsc rootfs: %w", imageRef, err)
	}
	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	reader := mutate.Extract(img)
	defer reader.Close()
	if err := untarRootfs(reader, destRootfs); err != nil {
		return nil, err
	}
	return &imageData{Config: cfg, Digest: digest.String()}, nil
}

func parseImageRef(imageRef string) (name.Reference, error) {
	opts := []name.Option{name.WeakValidation}
	if strings.Contains(imageRef, "localhost") || strings.Contains(imageRef, "127.0.0.1") || strings.Contains(imageRef, "registry:") {
		opts = append(opts, name.Insecure)
	}
	return name.ParseReference(imageRef, opts...)
}

func untarRootfs(reader io.Reader, destDir string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(destDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			linkTarget, err := safeJoin(destDir, header.Linkname)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
}

func (r *Runtime) waitForDaemonRunning(ctx context.Context, state *SandboxRuntimeState, authToken *string) (string, error) {
	target, err := url.Parse(fmt.Sprintf("http://%s/version", net.JoinHostPort(state.IP, defaultDaemonPort)))
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: time.Second}
	timeout := time.Duration(r.daemonStartTimeoutSec) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("timeout waiting for daemon to start")
		default:
			version, err := getDaemonVersion(timeoutCtx, target, client)
			if err != nil {
				time.Sleep(25 * time.Millisecond)
				continue
			}
			if authToken != nil && *authToken != "" {
				otelClient := &http.Client{
					Timeout:   time.Second,
					Transport: otelhttp.NewTransport(http.DefaultTransport),
				}
				go func() {
					if err := r.initializeDaemon(context.WithoutCancel(ctx), state.IP, *authToken, otelClient); err != nil {
						r.logger.ErrorContext(context.WithoutCancel(ctx), "Failed to initialize daemon telemetry", "error", err)
					}
				}()
			}
			return version, nil
		}
	}
}

func getDaemonVersion(ctx context.Context, target *url.URL, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("daemon returned status %d", resp.StatusCode)
	}
	var version struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", err
	}
	return version.Version, nil
}

func (r *Runtime) initializeDaemon(ctx context.Context, ip string, token string, client *http.Client) error {
	if !r.initializeDaemonTelemetry {
		return nil
	}
	body, err := json.Marshal(struct {
		Token string `json:"token"`
	}{Token: token})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s/init", net.JoinHostPort(ip, defaultDaemonPort)), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned non-200 status code: %d", resp.StatusCode)
	}
	return nil
}

func (r *Runtime) isRunning(ctx context.Context, sandboxID string) (bool, error) {
	state, err := r.client.State(ctx, sandboxID)
	if err != nil {
		return false, err
	}
	return state.Status == "running" || state.Status == "created", nil
}

func (r *Runtime) saveState(state *SandboxRuntimeState) error {
	state.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(r.sandboxDir(state.ID), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.statePath(state.ID), data, 0644)
}

func (r *Runtime) loadState(id string) (*SandboxRuntimeState, error) {
	data, err := os.ReadFile(r.statePath(id))
	if err != nil {
		return nil, err
	}
	var state SandboxRuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *Runtime) sandboxDir(id string) string {
	return filepath.Join(r.sandboxesDir, safePathPart(id))
}

func (r *Runtime) statePath(id string) string {
	return filepath.Join(r.sandboxDir(id), "state.json")
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func envSliceToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if ok && key != "" {
			result[key] = val
		}
	}
	return result
}

func envMapToSlice(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key, value := range values {
		if key == "" {
			continue
		}
		result = append(result, key+"="+value)
	}
	slices.Sort(result)
	return result
}

func mergeEnv(values ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, value := range values {
		for k, v := range value {
			result[k] = v
		}
	}
	return result
}

func mergeLabels(base map[string]string, extra map[string]string, name string, authToken string) map[string]string {
	result := map[string]string{}
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	if name != "" {
		result["daytona.sandbox_name"] = name
	}
	if authToken != "" {
		result["daytona.auth_token"] = authToken
	}
	return result
}

func sandboxEnv(sandbox dto.CreateSandboxDTO) map[string]string {
	values := map[string]string{
		"DAYTONA_SANDBOX_ID":       sandbox.Id,
		"DAYTONA_SANDBOX_SNAPSHOT": sandbox.Snapshot,
		"DAYTONA_SANDBOX_USER":     sandbox.OsUser,
	}
	if sandbox.OtelEndpoint != nil && *sandbox.OtelEndpoint != "" {
		values["DAYTONA_OTEL_ENDPOINT"] = *sandbox.OtelEndpoint
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func ensureFile(path string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE, mode)
	if err != nil {
		return err
	}
	return file.Close()
}

func safeJoin(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("path must be relative: %s", name)
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(cleanRoot, name))
	if err != nil {
		return "", err
	}
	if target != cleanRoot && !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes destination: %s", name)
	}
	return target, nil
}

func int64Ptr(v int64) *int64 {
	return &v
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func defaultLinuxDevices() []specs.LinuxDevice {
	return []specs.LinuxDevice{
		{Path: "/dev/null", Type: "c", Major: 1, Minor: 3, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/zero", Type: "c", Major: 1, Minor: 5, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/full", Type: "c", Major: 1, Minor: 7, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/random", Type: "c", Major: 1, Minor: 8, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/urandom", Type: "c", Major: 1, Minor: 9, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/tty", Type: "c", Major: 5, Minor: 0, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/console", Type: "c", Major: 5, Minor: 1, FileMode: fileModePtr(0600), UID: uint32Ptr(0), GID: uint32Ptr(0)},
		{Path: "/dev/ptmx", Type: "c", Major: 5, Minor: 2, FileMode: fileModePtr(0666), UID: uint32Ptr(0), GID: uint32Ptr(0)},
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func fileModePtr(v os.FileMode) *os.FileMode {
	return &v
}

func shortID(id string) string {
	value := safePathPart(id)
	if len(value) > 8 {
		return value[:8]
	}
	return value
}

func (r *Runtime) ensureBridge(ctx context.Context) error {
	if _, err := os.Stat("/proc/sys/net/ipv4/ip_forward"); err == nil {
		_ = os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	}

	link, err := netlink.LinkByName(r.bridgeName)
	if err != nil {
		bridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: r.bridgeName}}
		if err := netlink.LinkAdd(bridge); err != nil && !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create runsc bridge: %w", err)
		}
		link, err = netlink.LinkByName(r.bridgeName)
		if err != nil {
			return err
		}
	}
	addr := &netlink.Addr{IPNet: r.bridgeCIDR}
	if err := netlink.AddrAdd(link, addr); err != nil && !strings.Contains(err.Error(), "file exists") {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, "iptables", "-t", "nat", "-C", "POSTROUTING", "-s", r.bridgeCIDR.String(), "!", "-o", r.bridgeName, "-j", "MASQUERADE").Run(); err != nil {
		if err := exec.CommandContext(ctx, "iptables", "-t", "nat", "-A", "POSTROUTING", "-s", r.bridgeCIDR.String(), "!", "-o", r.bridgeName, "-j", "MASQUERADE").Run(); err != nil {
			r.logger.WarnContext(ctx, "Failed to ensure runsc bridge NAT rule", "error", err)
		}
	}
	return nil
}

func (r *Runtime) setupNetwork(ctx context.Context, state *SandboxRuntimeState) error {
	if err := r.ensureBridge(ctx); err != nil {
		return fmt.Errorf("ensure runsc bridge: %w", err)
	}
	if state.IP == "" {
		state.IP = r.allocateIP(state.ID)
	}
	state.NetNSName = "daytona-" + shortID(state.ID)
	state.NetNSPath = filepath.Join("/var/run/netns", state.NetNSName)
	state.HostVeth = "dt" + shortID(state.ID) + "h"
	peerName := "dt" + shortID(state.ID) + "c"
	if len(state.HostVeth) > 15 {
		state.HostVeth = state.HostVeth[:15]
	}
	if len(peerName) > 15 {
		peerName = peerName[:15]
	}

	if _, err := os.Stat(state.NetNSPath); os.IsNotExist(err) {
		if err := createNamedNetNS(state.NetNSName); err != nil {
			return err
		}
	}

	if _, err := netlink.LinkByName(state.HostVeth); err != nil {
		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{Name: state.HostVeth},
			PeerName:  peerName,
		}
		if err := netlink.LinkAdd(veth); err != nil {
			return fmt.Errorf("create veth: %w", err)
		}
		peer, err := netlink.LinkByName(peerName)
		if err != nil {
			return fmt.Errorf("lookup peer veth %s: %w", peerName, err)
		}
		ns, err := netns.GetFromName(state.NetNSName)
		if err != nil {
			return fmt.Errorf("open netns %s: %w", state.NetNSName, err)
		}
		if err := netlink.LinkSetNsFd(peer, int(ns)); err != nil {
			ns.Close()
			return fmt.Errorf("move peer veth %s into netns %s: %w", peerName, state.NetNSName, err)
		}
		ns.Close()
	}

	host, err := netlink.LinkByName(state.HostVeth)
	if err != nil {
		return fmt.Errorf("lookup host veth %s: %w", state.HostVeth, err)
	}
	bridge, err := netlink.LinkByName(r.bridgeName)
	if err != nil {
		return fmt.Errorf("lookup bridge %s: %w", r.bridgeName, err)
	}
	_ = netlink.LinkSetMaster(host, bridge)
	if err := netlink.LinkSetUp(host); err != nil {
		return fmt.Errorf("set host veth %s up: %w", state.HostVeth, err)
	}
	return r.configurePeer(ctx, state, peerName)
}

func createNamedNetNS(name string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	originalNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("get original netns: %w", err)
	}
	defer originalNS.Close()

	newNS, err := netns.NewNamed(name)
	if err != nil {
		_ = netns.Set(originalNS)
		return fmt.Errorf("create netns %s: %w", name, err)
	}
	defer newNS.Close()

	if err := netns.Set(originalNS); err != nil {
		return fmt.Errorf("restore original netns after creating %s: %w", name, err)
	}
	return nil
}

func (r *Runtime) configurePeer(ctx context.Context, state *SandboxRuntimeState, peerName string) error {
	sandboxNS, err := netns.GetFromName(state.NetNSName)
	if err != nil {
		return fmt.Errorf("open netns %s: %w", state.NetNSName, err)
	}
	defer sandboxNS.Close()

	sandboxHandle, err := netlink.NewHandleAt(sandboxNS)
	if err != nil {
		return fmt.Errorf("create netlink handle for netns %s: %w", state.NetNSName, err)
	}
	defer sandboxHandle.Delete()

	peer, err := sandboxHandle.LinkByName(peerName)
	if err != nil {
		if peer, err = sandboxHandle.LinkByName("eth0"); err != nil {
			return fmt.Errorf("lookup peer veth %s or eth0 in netns %s: %w", peerName, state.NetNSName, err)
		}
	} else if err := sandboxHandle.LinkSetName(peer, "eth0"); err != nil {
		return fmt.Errorf("rename peer veth %s to eth0 in netns %s: %w", peerName, state.NetNSName, err)
	}
	peer, err = sandboxHandle.LinkByName("eth0")
	if err != nil {
		return fmt.Errorf("lookup eth0 in netns %s: %w", state.NetNSName, err)
	}
	ipNet := &net.IPNet{IP: net.ParseIP(state.IP), Mask: r.bridgeCIDR.Mask}
	if err := sandboxHandle.AddrAdd(peer, &netlink.Addr{IPNet: ipNet}); err != nil && !strings.Contains(err.Error(), "file exists") {
		return fmt.Errorf("assign %s to eth0 in netns %s: %w", ipNet.String(), state.NetNSName, err)
	}
	if err := sandboxHandle.LinkSetUp(peer); err != nil {
		return fmt.Errorf("set eth0 up in netns %s: %w", state.NetNSName, err)
	}
	if lo, err := sandboxHandle.LinkByName("lo"); err == nil {
		_ = sandboxHandle.LinkSetUp(lo)
	}
	if err := sandboxHandle.RouteAdd(&netlink.Route{
		LinkIndex: peer.Attrs().Index,
		Gw:        r.bridgeIP,
	}); err != nil && !strings.Contains(err.Error(), "file exists") {
		return fmt.Errorf("add default route via %s in netns %s: %w", r.bridgeIP.String(), state.NetNSName, err)
	}
	return nil
}

func (r *Runtime) teardownNetwork(ctx context.Context, state *SandboxRuntimeState) {
	if state.HostVeth != "" {
		if link, err := netlink.LinkByName(state.HostVeth); err == nil {
			_ = netlink.LinkDel(link)
		}
	}
	if state.NetNSName != "" {
		_ = netns.DeleteNamed(state.NetNSName)
	}
}

func (r *Runtime) allocateIP(id string) string {
	sum := sha256.Sum256([]byte(id))
	third := int(sum[0])%250 + 1
	fourth := int(binary.BigEndian.Uint16(sum[1:3]))%250 + 2
	base := r.bridgeIP.To4()
	if base == nil {
		return fmt.Sprintf("172.29.%d.%d", third, fourth)
	}
	return fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], third, fourth)
}

func boolToString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parseUIDGID(user string) (uint32, uint32) {
	if user == "" || user == "root" {
		return 0, 0
	}
	if uid, err := strconv.ParseUint(user, 10, 32); err == nil {
		return uint32(uid), uint32(uid)
	}
	return 0, 0
}
