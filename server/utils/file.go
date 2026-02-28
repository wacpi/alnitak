package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 判断文件是否存在
func IsFileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// CalculateFileHash 计算文件的 SHA256 哈希
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// MergeChunks 流式合并文件分片
func MergeChunks(dir string, totalChunks int, outputFile string) error {
	out, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer out.Close()

	for i := 0; i < totalChunks; i++ {
		partPath := filepath.Join(dir, fmt.Sprintf("%d.part", i))
		partFile, err := os.Open(partPath)
		if err != nil {
			return fmt.Errorf("打开分片失败: %w", err)
		}

		_, err = io.Copy(out, partFile)
		partFile.Close()
		if err != nil {
			return fmt.Errorf("写入合并文件失败: %w", err)
		}
	}
	return nil
}

// GetFileSuffix 从文件名中提取后缀（包含点）
func GetFileSuffix(fileName string) string {
	ext := filepath.Ext(fileName)
	return ext
}
