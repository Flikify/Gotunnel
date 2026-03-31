package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateTLSConfig 生成内存中的自签名证书并返回 TLS 配置
// 证书不限定具体 IP 地址，客户端使用 InsecureSkipVerify 跳过主机名验证（类似 frp）
func GenerateTLSConfig() (*tls.Config, error) {
	certPEM, keyPEM, err := GenerateTLSCertificatePEM()
	if err != nil {
		return nil, err
	}
	return TLSConfigFromPEM(certPEM, keyPEM)
}

// GenerateTLSCertificatePEM 生成自签名证书和私钥的 PEM 编码内容。
func GenerateTLSCertificatePEM() ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GoTunnel"},
			CommonName:   "GoTunnel Server",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		// 不限定 IP 地址和域名，客户端通过 InsecureSkipVerify + TOFU 验证
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	return certPEM, keyPEM, nil
}

// TLSConfigFromPEM 从 PEM 编码的证书和私钥创建 TLS 配置。
func TLSConfigFromPEM(certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// ClientTLSConfig 创建客户端 TLS 配置
func ClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}
}

// ClientTLSConfigWithTOFU 创建带 TOFU 验证的客户端 TLS 配置
// serverAddr: 服务器地址，用于存储指纹
// dataDir: 数据目录，用于存储指纹文件
// skipVerify: 是否跳过验证（测试环境使用）
func ClientTLSConfigWithTOFU(serverAddr, dataDir string, skipVerify bool) *tls.Config {
	if skipVerify {
		return &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}
	}

	return &tls.Config{
		InsecureSkipVerify: true, // 必须为 true，因为是自签名证书
		MinVersion:         tls.VersionTLS12,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return VerifyCertFingerprint(rawCerts, serverAddr, dataDir)
		},
	}
}

// CertFingerprint 计算证书指纹 (SHA256)
func CertFingerprint(certDER []byte) string {
	hash := sha256.Sum256(certDER)
	return hex.EncodeToString(hash[:])
}

// GetFingerprintPath 获取指纹文件路径
func GetFingerprintPath(serverAddr, dataDir string) string {
	// 将服务器地址转换为安全的文件名
	safeName := strings.ReplaceAll(serverAddr, ":", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	return filepath.Join(dataDir, ".fingerprint_"+safeName)
}

// LoadFingerprint 加载已保存的证书指纹
func LoadFingerprint(serverAddr, dataDir string) (string, error) {
	path := GetFingerprintPath(serverAddr, dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveFingerprint 保存证书指纹
func SaveFingerprint(serverAddr, dataDir, fingerprint string) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	path := GetFingerprintPath(serverAddr, dataDir)
	return os.WriteFile(path, []byte(fingerprint), 0600)
}

// VerifyCertFingerprint 验证证书指纹 (TOFU 模式)
func VerifyCertFingerprint(rawCerts [][]byte, serverAddr, dataDir string) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no certificate provided")
	}

	// 计算当前证书指纹
	currentFP := CertFingerprint(rawCerts[0])

	// 尝试加载已保存的指纹
	savedFP, err := LoadFingerprint(serverAddr, dataDir)
	if err != nil {
		// 首次连接，保存指纹
		if os.IsNotExist(err) {
			if saveErr := SaveFingerprint(serverAddr, dataDir, currentFP); saveErr != nil {
				return fmt.Errorf("failed to save fingerprint: %w", saveErr)
			}
			return nil // 首次连接，信任此证书
		}
		return fmt.Errorf("failed to load fingerprint: %w", err)
	}

	// 验证指纹是否匹配
	if savedFP != currentFP {
		return fmt.Errorf("certificate fingerprint mismatch: possible MITM attack\n"+
			"  Expected: %s\n  Got: %s\n"+
			"  If the server certificate was legitimately changed, delete: %s",
			savedFP, currentFP, GetFingerprintPath(serverAddr, dataDir))
	}

	return nil
}

// GenerateRandomHex 生成指定长度的随机十六进制字符串
func GenerateRandomHex(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}
