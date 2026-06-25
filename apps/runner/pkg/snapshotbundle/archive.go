// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package snapshotbundle

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const TarContentType = "application/x-tar"

func TarDirectory(srcDir, destPath string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return 0, fmt.Errorf("create tar parent directory: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("create tar %s: %w", destPath, err)
	}
	defer out.Close()

	tw := tar.NewWriter(out)
	defer tw.Close()

	if err := filepath.WalkDir(srcDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == srcDir {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, in)
			closeErr := in.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}); err != nil {
		return 0, fmt.Errorf("tar directory %s: %w", srcDir, err)
	}

	if err := tw.Close(); err != nil {
		return 0, fmt.Errorf("close tar writer: %w", err)
	}
	if err := out.Close(); err != nil {
		return 0, fmt.Errorf("close tar %s: %w", destPath, err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func UntarDirectory(srcPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create untar directory %s: %w", destDir, err)
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open tar %s: %w", srcPath, err)
	}
	defer in.Close()

	tr := tar.NewReader(in)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar %s: %w", srcPath, err)
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
			if err := os.Symlink(header.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		case tar.TypeLink:
			linkTarget, err := safeJoin(destDir, header.Linkname)
			if err != nil {
				return err
			}
			if err := os.Link(linkTarget, target); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
	return nil
}

func HashFiles(paths ...string) (string, error) {
	h := sha256.New()
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func safeJoin(root, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("tar path must be relative: %s", name)
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
		return "", fmt.Errorf("tar path escapes destination: %s", name)
	}
	return target, nil
}
