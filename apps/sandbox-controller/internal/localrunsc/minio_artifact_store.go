package localrunsc

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOArtifactStore struct{}

func (MinIOArtifactStore) Upload(ctx context.Context, storage computev1.LocalRunscStorageSpec, imagePath string, namespace string, name string) (string, error) {
	if !UsesObjectStorage(storage) {
		return imagePath, nil
	}
	client, bucket, prefix, err := minioClient(storage, namespace, name)
	if err != nil {
		return "", err
	}
	if err := ensureBucket(ctx, client, bucket); err != nil {
		return "", err
	}
	if err := filepath.WalkDir(imagePath, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(imagePath, path)
		if err != nil {
			return err
		}
		key := joinPath(prefix, filepath.ToSlash(rel))
		_, err = client.FPutObject(ctx, bucket, key, path, minio.PutObjectOptions{})
		return err
	}); err != nil {
		return "", err
	}
	return "s3://" + bucket + "/" + prefix, nil
}

func (MinIOArtifactStore) Download(ctx context.Context, storage computev1.LocalRunscStorageSpec, storageRef string, imagePath string) error {
	if !UsesObjectStorage(storage) {
		return nil
	}
	client, bucket, prefix, err := minioClientForRef(storage, storageRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(imagePath, 0o755); err != nil {
		return err
	}
	for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(object.Key, prefix), "/")
		if rel == "" {
			continue
		}
		target := filepath.Join(imagePath, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := client.FGetObject(ctx, bucket, object.Key, target, minio.GetObjectOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (MinIOArtifactStore) Delete(ctx context.Context, storage computev1.LocalRunscStorageSpec, storageRef string, imagePath string) error {
	if !UsesObjectStorage(storage) {
		return nil
	}
	client, bucket, prefix, err := minioClientForRef(storage, storageRef)
	if err != nil {
		return err
	}
	objects := make(chan minio.ObjectInfo)
	go func() {
		defer close(objects)
		for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				objects <- object
				return
			}
			objects <- minio.ObjectInfo{Key: object.Key}
		}
	}()
	for removeErr := range client.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			return removeErr.Err
		}
	}
	_ = imagePath
	return nil
}

func minioClient(storage computev1.LocalRunscStorageSpec, namespace string, name string) (*minio.Client, string, string, error) {
	if storage.Bucket == "" {
		return nil, "", "", fmt.Errorf("storage.bucket is required for s3 storage")
	}
	client, err := newMinioClient(storage)
	if err != nil {
		return nil, "", "", err
	}
	prefix := strings.Trim(storage.Prefix, "/")
	if namespace != "" {
		prefix = joinPath(prefix, namespace)
	}
	if name != "" {
		prefix = joinPath(prefix, name)
	}
	if prefix == "" {
		return nil, "", "", fmt.Errorf("storage prefix cannot be empty for checkpoint artifact upload")
	}
	return client, storage.Bucket, prefix, nil
}

func minioClientForRef(storage computev1.LocalRunscStorageSpec, storageRef string) (*minio.Client, string, string, error) {
	client, err := newMinioClient(storage)
	if err != nil {
		return nil, "", "", err
	}
	bucket := storage.Bucket
	if storageRef == "" {
		if bucket == "" {
			return nil, "", "", fmt.Errorf("storage.bucket is required for s3 storage")
		}
		if storage.Prefix == "" {
			return nil, "", "", fmt.Errorf("storageRef or storage.prefix is required")
		}
		return client, bucket, strings.Trim(storage.Prefix, "/"), nil
	}
	parsed, err := url.Parse(storageRef)
	if err != nil {
		return nil, "", "", err
	}
	if parsed.Scheme != "s3" {
		return nil, "", "", fmt.Errorf("unsupported storageRef scheme %q", parsed.Scheme)
	}
	if parsed.Host != "" {
		bucket = parsed.Host
	}
	if bucket == "" {
		return nil, "", "", fmt.Errorf("storage.bucket is required for s3 storage")
	}
	prefix := strings.Trim(parsed.Path, "/")
	if prefix == "" {
		return nil, "", "", fmt.Errorf("storageRef path is required")
	}
	return client, bucket, prefix, nil
}

func newMinioClient(storage computev1.LocalRunscStorageSpec) (*minio.Client, error) {
	endpoint, secure, err := minioEndpoint(storage.Endpoint)
	if err != nil {
		return nil, err
	}
	accessKey := firstEnv("MINIO_ACCESS_KEY", "MINIO_ROOT_USER", "AWS_ACCESS_KEY_ID")
	secretKey := firstEnv("MINIO_SECRET_KEY", "MINIO_ROOT_PASSWORD", "AWS_SECRET_ACCESS_KEY")
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("MinIO credentials are required through MINIO_ACCESS_KEY/MINIO_SECRET_KEY or MINIO_ROOT_USER/MINIO_ROOT_PASSWORD")
	}
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, os.Getenv("AWS_SESSION_TOKEN")),
		Secure: secure,
	})
}

func minioEndpoint(endpoint string) (string, bool, error) {
	if endpoint == "" {
		endpoint = "127.0.0.1:9000"
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", false, err
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}
	return endpoint, false, nil
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}

func firstEnv(names ...string) string {
	for _, name := range names {
		value := os.Getenv(name)
		if value != "" {
			return value
		}
	}
	return ""
}
