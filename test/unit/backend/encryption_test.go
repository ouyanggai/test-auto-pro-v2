package backend_test

import (
	"testing"

	"test-auto-pro-v2/internal/adapter/target"
)

func TestPasswordEncryptionIsDeterministicAndNeverReturnsPlaintext(t *testing.T) {
	password := generatedValue(t, 12)
	key := generatedValue(t, 8)
	first, err := target.EncryptPassword(password, key)
	if err != nil {
		t.Fatalf("加密失败：%v", err)
	}
	second, err := target.EncryptPassword(password, key)
	if err != nil {
		t.Fatalf("重复加密失败：%v", err)
	}
	if first == "" || first != second || first == password {
		t.Fatal("密码加密结果不符合 V1 已核实的确定性边界")
	}
}
