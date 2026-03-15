package utils

import "errors"

// Short ID 使用的 64 字符表（URL 安全）
// 与常见 Base64 URL 变体兼容，但不使用 '=' 作为填充。
const shortIDAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
const shortIDBase = uint64(len(shortIDAlphabet))

// EncodeUint64ToShortID 将一个 64 位无符号整数编码为固定长度的 11 位字符串。
// 设计目标：用于对外暴露的短 ID，例如稿件 ID、资源 ID 等。
func EncodeUint64ToShortID(id uint64) string {
	// 11 位 64 进制可覆盖 64^11 约 7.5e19 的空间，足够承载 Snowflake 等 64 位 ID。
	const length = 11

	// 特殊情况：id == 0 时直接返回全 'A'
	if id == 0 {
		return "AAAAAAAAAAA"
	}

	buf := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		mod := id % shortIDBase
		id /= shortIDBase
		buf[i] = shortIDAlphabet[mod]
	}

	return string(buf)
}

// DecodeShortIDToUint64 将 11 位 Short ID 还原为 64 位无符号整数。
func DecodeShortIDToUint64(s string) (uint64, error) {
	if len(s) != 11 {
		return 0, errors.New("invalid id length")
	}

	var result uint64
	for i := 0; i < len(s); i++ {
		ch := s[i]
		value := -1

		switch {
		case ch >= 'A' && ch <= 'Z':
			value = int(ch - 'A')
		case ch >= 'a' && ch <= 'z':
			value = int(ch-'a') + 26
		case ch >= '0' && ch <= '9':
			value = int(ch-'0') + 52
		case ch == '-':
			value = 62
		case ch == '_':
			value = 63
		}

		if value < 0 {
			return 0, errors.New("invalid character in id")
		}

		result = result*shortIDBase + uint64(value)
	}

	return result, nil
}

