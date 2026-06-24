package utils

import (
	"crypto/rand"
	"math/big"
	"strconv"
)

// 生成n位数字随机码
func GenerateNumberCode(length int) string {
	res := ""
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(10))
		res += strconv.Itoa(int(num.Int64()))
	}
	return res
}
