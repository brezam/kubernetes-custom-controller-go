package util

import (
	"math/rand"
	"time"
)

var random *rand.Rand

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func GenerateRandomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = chars[random.Intn(len(chars))]
	}
	return string(b)
}
