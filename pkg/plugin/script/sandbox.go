package script

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox 插件沙箱配置
type Sandbox struct {
	// 允许访问的路径列表（绝对路径）
	AllowedPaths []string
	// 允许写入的路径列表（必须是 AllowedPaths 的子集）
	WritablePaths []string
	// 禁止访问的路径（黑名单，优先级高于白名单）
	DeniedPaths []string
	// 是否允许网络访问
	AllowNetwork bool
	// 最大文件读取大小 (bytes)
	MaxReadSize int64
	// 最大文件写入大小 (bytes)
	MaxWriteSize int64
}

// DefaultSandbox 返回默认沙箱配置（最小权限）
func DefaultSandbox() *Sandbox {
	return &Sandbox{
		AllowedPaths:  []string{},
		WritablePaths: []string{},
		DeniedPaths:   defaultDeniedPaths(),
		AllowNetwork:  false,
		MaxReadSize:   10 * 1024 * 1024,  // 10MB
		MaxWriteSize:  1 * 1024 * 1024,   // 1MB
	}
}

// defaultDeniedPaths 返回默认禁止访问的路径
func defaultDeniedPaths() []string {
	home, _ := os.UserHomeDir()
	denied := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/root",
		"/.ssh",
		"/.gnupg",
		"/.aws",
		"/.kube",
		"/proc",
		"/sys",
	}
	if home != "" {
		denied = append(denied,
			filepath.Join(home, ".ssh"),
			filepath.Join(home, ".gnupg"),
			filepath.Join(home, ".aws"),
			filepath.Join(home, ".kube"),
			filepath.Join(home, ".config"),
			filepath.Join(home, ".local"),
		)
	}
	return denied
}

// ValidateReadPath 验证读取路径是否允许
func (s *Sandbox) ValidateReadPath(path string) error {
	return s.validatePath(path, false)
}

// ValidateWritePath 验证写入路径是否允许
func (s *Sandbox) ValidateWritePath(path string) error {
	return s.validatePath(path, true)
}

func (s *Sandbox) validatePath(path string, write bool) error {
	// 清理路径，防止路径遍历攻击
	cleanPath, err := s.cleanPath(path)
	if err != nil {
		return err
	}

	// 检查黑名单（优先级最高）
	if s.isDenied(cleanPath) {
		return fmt.Errorf("access denied: path is in denied list")
	}

	// 检查白名单
	allowedList := s.AllowedPaths
	if write {
		allowedList = s.WritablePaths
	}

	if len(allowedList) == 0 {
		return fmt.Errorf("access denied: no paths allowed")
	}

	if !s.isAllowed(cleanPath, allowedList) {
		if write {
			return fmt.Errorf("access denied: path not in writable list")
		}
		return fmt.Errorf("access denied: path not in allowed list")
	}

	return nil
}

// cleanPath 清理并验证路径
func (s *Sandbox) cleanPath(path string) (string, error) {
	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// 清理路径（解析 .. 和 .）
	cleanPath := filepath.Clean(absPath)

	// 检查符号链接（防止通过符号链接绕过限制）
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		// 文件可能不存在，使用清理后的路径
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		realPath = cleanPath
	}

	// 再次检查路径遍历
	if strings.Contains(realPath, "..") {
		return "", fmt.Errorf("path traversal detected")
	}

	return realPath, nil
}

// isDenied 检查路径是否在黑名单中
func (s *Sandbox) isDenied(path string) bool {
	for _, denied := range s.DeniedPaths {
		if strings.HasPrefix(path, denied) || path == denied {
			return true
		}
	}
	return false
}

// isAllowed 检查路径是否在白名单中
func (s *Sandbox) isAllowed(path string, allowedList []string) bool {
	for _, allowed := range allowedList {
		if strings.HasPrefix(path, allowed) || path == allowed {
			return true
		}
	}
	return false
}
