package localrunsc

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
)

const (
	DefaultRunscPath      = "runsc"
	DefaultArtifactRoot   = "/var/lib/daytona-localrunsc"
	DefaultStorageMode    = "filesystem"
	StorageModeS3         = "s3"
	StorageModeFilesystem = "filesystem"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

type Runtime struct {
	RunscPath    string
	RunscRoot    string
	ArtifactRoot string
	Runner       CommandRunner
	Store        ArtifactStore
}

type CheckpointRequest struct {
	Namespace string                          `json:"namespace,omitempty"`
	Name      string                          `json:"name,omitempty"`
	SandboxID string                          `json:"sandboxId"`
	ImagePath string                          `json:"imagePath,omitempty"`
	Storage   computev1.LocalRunscStorageSpec `json:"storage,omitempty"`
}

type CheckpointResult struct {
	ImagePath  string `json:"imagePath"`
	StorageRef string `json:"storageRef"`
}

type RestoreRequest struct {
	Namespace         string                          `json:"namespace,omitempty"`
	Name              string                          `json:"name,omitempty"`
	SandboxID         string                          `json:"sandboxId"`
	RestoredSandboxID string                          `json:"restoredSandboxId,omitempty"`
	ImagePath         string                          `json:"imagePath,omitempty"`
	StorageRef        string                          `json:"storageRef,omitempty"`
	Storage           computev1.LocalRunscStorageSpec `json:"storage,omitempty"`
}

type CleanupRequest struct {
	Namespace  string                          `json:"namespace,omitempty"`
	Name       string                          `json:"name,omitempty"`
	ImagePath  string                          `json:"imagePath,omitempty"`
	StorageRef string                          `json:"storageRef,omitempty"`
	Storage    computev1.LocalRunscStorageSpec `json:"storage,omitempty"`
}

type ArtifactStore interface {
	Upload(ctx context.Context, storage computev1.LocalRunscStorageSpec, imagePath string, namespace string, name string) (string, error)
	Download(ctx context.Context, storage computev1.LocalRunscStorageSpec, storageRef string, imagePath string) error
	Delete(ctx context.Context, storage computev1.LocalRunscStorageSpec, storageRef string, imagePath string) error
}

func NewRuntime(runscPath string, artifactRoot string, runner CommandRunner) *Runtime {
	if runscPath == "" {
		runscPath = DefaultRunscPath
	}
	if artifactRoot == "" {
		artifactRoot = DefaultArtifactRoot
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Runtime{
		RunscPath:    runscPath,
		ArtifactRoot: artifactRoot,
		Runner:       runner,
		Store:        MinIOArtifactStore{},
	}
}

func (r *Runtime) Checkpoint(ctx context.Context, req CheckpointRequest) (CheckpointResult, error) {
	if req.SandboxID == "" {
		return CheckpointResult{}, fmt.Errorf("sandboxId is required")
	}
	imagePath := req.ImagePath
	if imagePath == "" {
		root := r.ArtifactRoot
		if req.Storage.Path != "" {
			root = req.Storage.Path
		}
		imagePath = ArtifactPath(root, req.Namespace, req.Name)
	}
	if imagePath == "" {
		return CheckpointResult{}, fmt.Errorf("imagePath is required")
	}
	if err := os.MkdirAll(imagePath, 0o755); err != nil {
		return CheckpointResult{}, err
	}
	log.Printf("localrunsc checkpoint namespace=%q name=%q sandboxID=%q imagePath=%q", req.Namespace, req.Name, req.SandboxID, imagePath)
	if err := r.runsc(ctx, "checkpoint", "--leave-running", "--image-path", imagePath, req.SandboxID); err != nil {
		return CheckpointResult{}, err
	}
	storageRef := StorageRef(req.Storage, imagePath, req.Namespace, req.Name)
	if UsesObjectStorage(req.Storage) {
		log.Printf("localrunsc artifact upload namespace=%q name=%q storageRef=%q", req.Namespace, req.Name, storageRef)
		uploadedRef, err := r.Store.Upload(ctx, req.Storage, imagePath, req.Namespace, req.Name)
		if err != nil {
			return CheckpointResult{}, err
		}
		storageRef = uploadedRef
	}
	return CheckpointResult{
		ImagePath:  imagePath,
		StorageRef: storageRef,
	}, nil
}

func (r *Runtime) Restore(ctx context.Context, req RestoreRequest) error {
	if req.SandboxID == "" {
		return fmt.Errorf("sandboxId is required")
	}
	imagePath := req.ImagePath
	if imagePath == "" {
		root := r.ArtifactRoot
		if req.Storage.Path != "" {
			root = req.Storage.Path
		}
		imagePath = ArtifactPath(root, req.Namespace, req.Name)
	}
	if imagePath == "" {
		return fmt.Errorf("imagePath is required")
	}
	if UsesObjectStorage(req.Storage) {
		storageRef := req.StorageRef
		if storageRef == "" {
			storageRef = StorageRef(req.Storage, imagePath, req.Namespace, req.Name)
		}
		log.Printf("localrunsc artifact download namespace=%q name=%q storageRef=%q imagePath=%q", req.Namespace, req.Name, storageRef, imagePath)
		if err := r.Store.Download(ctx, req.Storage, storageRef, imagePath); err != nil {
			return err
		}
	}
	targetID := req.RestoredSandboxID
	if targetID == "" {
		targetID = req.SandboxID
	}
	log.Printf("localrunsc restore sandboxID=%q restoredSandboxID=%q imagePath=%q", req.SandboxID, targetID, imagePath)
	return r.runsc(ctx, "restore", "--image-path", imagePath, targetID)
}

func (r *Runtime) Cleanup(ctx context.Context, req CleanupRequest) error {
	imagePath := req.ImagePath
	if imagePath == "" {
		root := r.ArtifactRoot
		if req.Storage.Path != "" {
			root = req.Storage.Path
		}
		imagePath = ArtifactPath(root, req.Namespace, req.Name)
	}
	if req.StorageRef != "" || UsesObjectStorage(req.Storage) {
		log.Printf("localrunsc artifact cleanup namespace=%q name=%q storageRef=%q imagePath=%q", req.Namespace, req.Name, req.StorageRef, imagePath)
		if err := r.Store.Delete(ctx, req.Storage, req.StorageRef, imagePath); err != nil {
			return err
		}
	}
	if imagePath != "" {
		return os.RemoveAll(imagePath)
	}
	return nil
}

func (r *Runtime) runsc(ctx context.Context, args ...string) error {
	return r.Runner.Run(ctx, r.RunscPath, r.runscArgs(args...)...)
}

func (r *Runtime) runscArgs(args ...string) []string {
	if r.RunscRoot == "" {
		return args
	}
	withRoot := []string{"--root", r.RunscRoot}
	return append(withRoot, args...)
}

func ArtifactPath(root string, namespace string, name string) string {
	if root == "" || namespace == "" || name == "" {
		return ""
	}
	return filepath.Join(root, namespace, name)
}

func StorageRef(storage computev1.LocalRunscStorageSpec, imagePath string, namespace string, name string) string {
	mode := storage.Mode
	if mode == "" {
		mode = DefaultStorageMode
	}
	if mode == StorageModeS3 || storage.Bucket != "" {
		path := strings.Trim(storage.Prefix, "/")
		if namespace != "" {
			path = joinPath(path, namespace)
		}
		if name != "" {
			path = joinPath(path, name)
		}
		if path == "" {
			return "s3://" + storage.Bucket
		}
		return "s3://" + storage.Bucket + "/" + path
	}
	return imagePath
}

func UsesObjectStorage(storage computev1.LocalRunscStorageSpec) bool {
	return storage.Mode == StorageModeS3 || storage.Bucket != "" || storage.Endpoint != ""
}

func joinPath(base string, child string) string {
	if base == "" {
		return child
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(child, "/")
}
