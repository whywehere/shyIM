package utils

import (
	"crypto/md5"
	"fmt"
	"shyIM/config"
)

// GetMD5 加盐生成 md5
func GetMD5(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s+config.GlobalConfig.APP.Salt)))
}
