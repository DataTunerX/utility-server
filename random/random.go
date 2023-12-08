package random

import (
	"math/rand"
	"time"
	"unsafe"
)

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var randomSource = rand.NewSource(time.Now().UnixNano())

const (
	bitsPerChar    = 6
	charIndexMask  = 1<<bitsPerChar - 1
	maxCharsPerInt = 63 / bitsPerChar
)

func GenerateRandomString(length int) string {
	randomBytes := make([]byte, length)
	for i, cache, remain := length-1, randomSource.Int63(), maxCharsPerInt; i >= 0; {
		if remain == 0 {
			cache, remain = randomSource.Int63(), maxCharsPerInt
		}
		if idx := int(cache & charIndexMask); idx < len(alphabet) {
			randomBytes[i] = alphabet[idx]
			i--
		}
		cache >>= bitsPerChar
		remain--
	}
	return *(*string)(unsafe.Pointer(&randomBytes))
}
