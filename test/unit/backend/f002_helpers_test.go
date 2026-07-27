package backend_test

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func generatedValue(t *testing.T, byteCount int) string {
	t.Helper()
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		t.Fatal("无法生成测试期临时值")
	}
	return hex.EncodeToString(data)
}
