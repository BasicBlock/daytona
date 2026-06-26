package localrunsc

import (
	"context"
	"reflect"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
)

func TestRuntimeCheckpointRunsRunscWithImagePath(t *testing.T) {
	runner := &recordingRunner{}
	runtime := NewRuntime("/usr/local/bin/runsc", t.TempDir(), runner)

	result, err := runtime.Checkpoint(context.Background(), CheckpointRequest{
		Namespace: "sandboxes",
		Name:      "warm",
		SandboxID: "sandbox-agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ImagePath == "" || result.StorageRef != result.ImagePath {
		t.Fatalf("expected filesystem storage ref to image path, got %#v", result)
	}
	want := []string{"checkpoint", "--leave-running", "--image-path", result.ImagePath, "sandbox-agent"}
	if runner.name != "/usr/local/bin/runsc" || !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected command: %s %#v", runner.name, runner.args)
	}
}

func TestRuntimeCheckpointUsesConfiguredRunscRoot(t *testing.T) {
	runner := &recordingRunner{}
	runtime := NewRuntime("runsc", t.TempDir(), runner)
	runtime.RunscRoot = "/run/containerd/runsc/k8s.io"

	result, err := runtime.Checkpoint(context.Background(), CheckpointRequest{
		Namespace: "sandboxes",
		Name:      "warm",
		SandboxID: "sandbox-agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--root", "/run/containerd/runsc/k8s.io", "checkpoint", "--leave-running", "--image-path", result.ImagePath, "sandbox-agent"}
	if runner.name != "runsc" || !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected command: %s %#v", runner.name, runner.args)
	}
}

func TestRuntimeCheckpointUsesStoragePathAndS3Ref(t *testing.T) {
	runner := &recordingRunner{}
	store := &recordingStore{uploadRef: "s3://daytona-local/snapshots/sandboxes/warm"}
	runtime := NewRuntime("runsc", "/unused", runner)
	runtime.Store = store

	result, err := runtime.Checkpoint(context.Background(), CheckpointRequest{
		Namespace: "sandboxes",
		Name:      "warm",
		SandboxID: "sandbox-agent",
		Storage: computev1.LocalRunscStorageSpec{
			Mode:   StorageModeS3,
			Path:   t.TempDir(),
			Bucket: "daytona-local",
			Prefix: "snapshots",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StorageRef != "s3://daytona-local/snapshots/sandboxes/warm" {
		t.Fatalf("unexpected storage ref %q", result.StorageRef)
	}
	if !store.uploaded {
		t.Fatal("expected checkpoint artifacts to be uploaded")
	}
}

func TestRuntimeRestoreRunsRunscRestore(t *testing.T) {
	runner := &recordingRunner{}
	runtime := NewRuntime("runsc", t.TempDir(), runner)

	err := runtime.Restore(context.Background(), RestoreRequest{
		SandboxID:         "sandbox-agent",
		RestoredSandboxID: "sandbox-fork",
		ImagePath:         "/snapshots/warm",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"restore", "--image-path", "/snapshots/warm", "sandbox-fork"}
	if runner.name != "runsc" || !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected command: %s %#v", runner.name, runner.args)
	}
}

func TestRuntimeRestoreDownloadsS3Artifacts(t *testing.T) {
	runner := &recordingRunner{}
	store := &recordingStore{}
	runtime := NewRuntime("runsc", t.TempDir(), runner)
	runtime.Store = store

	err := runtime.Restore(context.Background(), RestoreRequest{
		Namespace:  "sandboxes",
		Name:       "warm",
		SandboxID:  "sandbox-agent",
		StorageRef: "s3://daytona-local/snapshots/sandboxes/warm",
		Storage: computev1.LocalRunscStorageSpec{
			Mode:   StorageModeS3,
			Bucket: "daytona-local",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !store.downloaded {
		t.Fatal("expected restore artifacts to be downloaded")
	}
	if runner.args[0] != "restore" || runner.args[1] != "--image-path" || runner.args[2] == "" {
		t.Fatalf("unexpected restore command: %#v", runner.args)
	}
}

func TestRuntimeCleanupDeletesS3AndLocalArtifacts(t *testing.T) {
	runner := &recordingRunner{}
	store := &recordingStore{}
	root := t.TempDir()
	runtime := NewRuntime("runsc", root, runner)
	runtime.Store = store

	err := runtime.Cleanup(context.Background(), CleanupRequest{
		Namespace:  "sandboxes",
		Name:       "warm",
		StorageRef: "s3://daytona-local/snapshots/sandboxes/warm",
		Storage: computev1.LocalRunscStorageSpec{
			Mode:   StorageModeS3,
			Bucket: "daytona-local",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !store.deleted {
		t.Fatal("expected object store artifacts to be deleted")
	}
}

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return nil
}

type recordingStore struct {
	uploadRef  string
	uploaded   bool
	downloaded bool
	deleted    bool
}

func (s *recordingStore) Upload(_ context.Context, _ computev1.LocalRunscStorageSpec, _ string, _ string, _ string) (string, error) {
	s.uploaded = true
	return s.uploadRef, nil
}

func (s *recordingStore) Download(_ context.Context, _ computev1.LocalRunscStorageSpec, _ string, _ string) error {
	s.downloaded = true
	return nil
}

func (s *recordingStore) Delete(_ context.Context, _ computev1.LocalRunscStorageSpec, _ string, _ string) error {
	s.deleted = true
	return nil
}
