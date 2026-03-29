package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

func safeArchivePath(destDir, entryName string) (string, error) {
	cleanDest := filepath.Clean(destDir)
	cleanEntry := filepath.Clean(entryName)

	if cleanEntry == "." {
		return "", nil
	}
	if cleanEntry == string(os.PathSeparator) {
		return "", fmt.Errorf("invalid archive entry path %q", entryName)
	}
	if filepath.IsAbs(cleanEntry) {
		return "", fmt.Errorf("archive entry %q uses absolute path", entryName)
	}

	targetPath := filepath.Join(cleanDest, cleanEntry)
	relPath, err := filepath.Rel(cleanDest, targetPath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes destination", entryName)
	}

	return targetPath, nil
}

func binaryCandidateNames(component, goos string) []string {
	names := []string{component}
	if goos == "windows" {
		names = append(names, component+".exe")
	}
	return names
}

func findExtractedBinary(extractDir, component, goos string) (string, error) {
	var binaryPath string
	legacyPrefix := "gotunnel-" + component
	candidateNames := binaryCandidateNames(component, goos)

	err := filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		name := info.Name()
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") {
			return nil
		}

		// 优先匹配当前发布归档中的标准二进制名，其次兼容旧的 gotunnel-{component}-* 命名。
		if slices.Contains(candidateNames, name) || strings.HasPrefix(name, legacyPrefix) {
			binaryPath = path
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if binaryPath == "" {
		return "", fmt.Errorf("binary not found in archive")
	}

	return binaryPath, nil
}

// GetArchiveExt 根据 URL 获取压缩包扩展名
func GetArchiveExt(url string) string {
	if strings.HasSuffix(url, ".tar.gz") {
		return ".tar.gz"
	}
	if strings.HasSuffix(url, ".zip") {
		return ".zip"
	}
	// 默认根据平台
	if runtime.GOOS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// ExtractArchive 解压压缩包
func ExtractArchive(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return ExtractTarGz(archivePath, destDir)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return ExtractZip(archivePath, destDir)
	}
	return fmt.Errorf("unsupported archive format")
}

// ExtractTarGz 解压 tar.gz 文件
func ExtractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath, err := safeArchivePath(destDir, header.Name)
		if err != nil {
			return err
		}
		if targetPath == "" {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// ExtractZip 解压 zip 文件
func ExtractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath, err := safeArchivePath(destDir, file.Name)
		if err != nil {
			return err
		}
		if targetPath == "" {
			continue
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			return err
		}

		dstFile, err := os.Create(targetPath)
		if err != nil {
			srcFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// FindExtractedBinary 在解压目录中查找可执行文件
func FindExtractedBinary(extractDir, component string) (string, error) {
	return findExtractedBinary(extractDir, component, runtime.GOOS)
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// DownloadFile 下载文件
func DownloadFile(url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// DownloadAndExtract 下载并解压压缩包，返回解压后的可执行文件路径
func DownloadAndExtract(downloadURL, component string) (binaryPath string, cleanup func(), err error) {
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102150405")
	archivePath := filepath.Join(tempDir, "gotunnel_update_"+timestamp+GetArchiveExt(downloadURL))

	if err := DownloadFile(downloadURL, archivePath); err != nil {
		return "", nil, fmt.Errorf("download update: %w", err)
	}

	extractDir := filepath.Join(tempDir, "gotunnel_extract_"+timestamp)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		os.Remove(archivePath)
		return "", nil, fmt.Errorf("create extract dir: %w", err)
	}

	cleanup = func() {
		os.Remove(archivePath)
		os.RemoveAll(extractDir)
	}

	if err := ExtractArchive(archivePath, extractDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extract archive: %w", err)
	}

	binaryPath, err = FindExtractedBinary(extractDir, component)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("find binary: %w", err)
	}

	// 设置执行权限
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryPath, 0755); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("chmod: %w", err)
		}
	}

	return binaryPath, cleanup, nil
}
