package main

import (
	"crypto/md5"
	"fmt"
)

func md5sum(x []byte) string {
	return fmt.Sprintf("%x", md5.Sum(x))
}
