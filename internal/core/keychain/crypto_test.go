package keychain

import (
	"bytes"
	"testing"
)

// --- master key 生成 ---

func TestGenerateMasterKey(t *testing.T) {
	key, err := generateMasterKey()
	if err != nil {
		t.Fatalf("generateMasterKey() error: %v", err)
	}
	if len(key) != masterKeySize {
		t.Fatalf("key length = %d, want %d", len(key), masterKeySize)
	}
	if bytes.Equal(key, make([]byte, masterKeySize)) {
		t.Fatal("generated key is all zeros")
	}
}

// --- AES-256-GCM 加解密 ---

func TestEncryptDecrypt(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte(`{"access_token":"test-token","secret":"test-secret"}`)

	cipherData, err := encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("encrypt() error: %v", err)
	}

	expectedMinLen := nonceSize + len(plaintext)
	if len(cipherData) < expectedMinLen {
		t.Fatalf("cipherData length = %d, want >= %d", len(cipherData), expectedMinLen)
	}

	if bytes.Contains(cipherData, plaintext) {
		t.Fatal("ciphertext must not contain plaintext verbatim")
	}

	decrypted, err := decrypt(key, cipherData, nil)
	if err != nil {
		t.Fatalf("decrypt() error: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDifferentNonce(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("same plaintext for nonce uniqueness test")

	c1, err := encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("first encrypt() error: %v", err)
	}
	c2, err := encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("second encrypt() error: %v", err)
	}

	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertext (random nonce)")
	}
}

func TestDecryptTamperedData(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("integrity check test data")

	cipherData, _ := encrypt(key, plaintext, nil)

	tampered := make([]byte, len(cipherData))
	copy(tampered, cipherData)
	tampered[nonceSize+1] ^= 0xFF

	_, err := decrypt(key, tampered, nil)
	if err == nil {
		t.Fatal("decrypt should fail on tampered data (AEAD integrity)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := generateMasterKey()
	key2, _ := generateMasterKey()
	plaintext := []byte("wrong key test data")

	cipherData, _ := encrypt(key1, plaintext, nil)

	_, err := decrypt(key2, cipherData, nil)
	if err == nil {
		t.Fatal("decrypt should fail with wrong key")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key, _ := generateMasterKey()

	_, err := decrypt(key, []byte("short"), nil)
	if err == nil {
		t.Fatal("decrypt should fail on data shorter than nonce size")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	key, _ := generateMasterKey()

	cipherData, err := encrypt(key, []byte{}, nil)
	if err != nil {
		t.Fatalf("encrypt empty plaintext error: %v", err)
	}

	expectedLen := nonceSize + 16
	if len(cipherData) != expectedLen {
		t.Fatalf("cipherData length = %d, want %d", len(cipherData), expectedLen)
	}

	decrypted, err := decrypt(key, cipherData, nil)
	if err != nil {
		t.Fatalf("decrypt empty ciphertext error: %v", err)
	}
	if len(decrypted) != 0 {
		t.Fatalf("decrypted = %q, want empty", decrypted)
	}
}

func TestEncryptLargeData(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := make([]byte, 1<<20)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	cipherData, err := encrypt(key, plaintext, nil)
	if err != nil {
		t.Fatalf("encrypt 1MB data error: %v", err)
	}

	decrypted, err := decrypt(key, cipherData, nil)
	if err != nil {
		t.Fatalf("decrypt 1MB data error: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("decrypted data mismatch for large payload")
	}
}

func TestEncryptInvalidKeySize(t *testing.T) {
	badKey := make([]byte, 15)
	_, err := encrypt(badKey, []byte("data"), nil)
	if err == nil {
		t.Fatal("encrypt should fail with invalid key size")
	}
}

func TestDecryptInvalidKeySize(t *testing.T) {
	badKey := make([]byte, 15)
	fakeData := make([]byte, nonceSize+16+10)
	_, err := decrypt(badKey, fakeData, nil)
	if err == nil {
		t.Fatal("decrypt should fail with invalid key size")
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte("sensitive-data-to-clear")
	zeroBytes(data)
	for i, b := range data {
		if b != 0 {
			t.Fatalf("byte[%d] = %d, want 0", i, b)
		}
	}
}

// --- AAD (SEC-009) ---

func TestEncryptDecryptWithAAD(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("test data with AAD binding")
	aad := []byte("sdk-account-key")

	cipherData, err := encrypt(key, plaintext, aad)
	if err != nil {
		t.Fatalf("encrypt with AAD error: %v", err)
	}

	decrypted, err := decrypt(key, cipherData, aad)
	if err != nil {
		t.Fatalf("decrypt with same AAD error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongAAD(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("cross-account replacement test")

	cipherData, _ := encrypt(key, plaintext, []byte("correct-account"))

	_, err := decrypt(key, cipherData, []byte("wrong-account"))
	if err == nil {
		t.Fatal("decrypt should fail with mismatched AAD (cross-account attack)")
	}
}

func TestDecryptNilAADvsSetAAD(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("AAD presence test")

	cipherData, _ := encrypt(key, plaintext, []byte("some-aad"))

	_, err := decrypt(key, cipherData, nil)
	if err == nil {
		t.Fatal("decrypt with nil AAD should fail when encrypted with non-nil AAD")
	}
}

func TestAADBackwardCompat(t *testing.T) {
	key, _ := generateMasterKey()
	plaintext := []byte("old format data without AAD")

	cipherData, _ := encrypt(key, plaintext, nil)

	_, err := decrypt(key, cipherData, []byte("new-aad"))
	if err == nil {
		t.Fatal("decrypt with new AAD should fail on old-format ciphertext (nil AAD)")
	}

	decrypted, err := decrypt(key, cipherData, nil)
	if err != nil {
		t.Fatalf("decrypt with nil AAD should succeed on old-format: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

// --- decryptWithAADFallback（SEC-009 跨平台降级逻辑） ---

func TestFallback_NewFormatDirectSuccess(t *testing.T) {
	key, _ := generateMasterKey()
	aad := []byte("sdk-account")
	plain := []byte(`{"token":"new-format"}`)

	cipher, _ := encrypt(key, plain, aad)

	result, needsMigration, err := decryptWithAADFallback(key, cipher, aad)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsMigration {
		t.Fatal("new-format ciphertext should not need migration")
	}
	if !bytes.Equal(result, plain) {
		t.Fatalf("plaintext mismatch: got %q", result)
	}
}

func TestFallback_OldFormatNeedsMigration(t *testing.T) {
	key, _ := generateMasterKey()
	aad := []byte("sdk-account")
	plain := []byte(`{"token":"old-format-no-aad"}`)

	cipher, _ := encrypt(key, plain, nil)

	result, needsMigration, err := decryptWithAADFallback(key, cipher, aad)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsMigration {
		t.Fatal("old-format ciphertext (nil AAD) should need migration")
	}
	if !bytes.Equal(result, plain) {
		t.Fatalf("plaintext mismatch: got %q", result)
	}
}

func TestFallback_WrongKeyFailsBoth(t *testing.T) {
	key1, _ := generateMasterKey()
	key2, _ := generateMasterKey()
	plain := []byte("secret data")

	cipher, _ := encrypt(key1, plain, []byte("aad"))

	_, _, err := decryptWithAADFallback(key2, cipher, []byte("aad"))
	if err == nil {
		t.Fatal("should fail when master key is wrong (both AAD and nil paths)")
	}
}

func TestFallback_CrossAccountBlocked(t *testing.T) {
	key, _ := generateMasterKey()
	plain := []byte(`{"token":"belongs-to-A"}`)

	cipherA, _ := encrypt(key, plain, []byte("account-A"))

	// SEC-009 核心安全属性：跨账号 .enc 替换攻击被 GCM 认证拒绝
	_, _, err := decryptWithAADFallback(key, cipherA, []byte("account-B"))
	if err == nil {
		t.Fatal("cross-account ciphertext should be rejected by both AAD and nil-AAD paths")
	}
}

func TestFallback_MigratedDataNoLongerNeedsMigration(t *testing.T) {
	key, _ := generateMasterKey()
	aad := []byte("sdk-123")
	plain := []byte(`{"token":"migrate-me"}`)

	oldCipher, _ := encrypt(key, plain, nil)

	result, needsMigration, err := decryptWithAADFallback(key, oldCipher, aad)
	if err != nil || !needsMigration {
		t.Fatalf("first read: err=%v, needsMigration=%v", err, needsMigration)
	}

	newCipher, _ := encrypt(key, result, aad)

	result2, needsMigration2, err := decryptWithAADFallback(key, newCipher, aad)
	if err != nil {
		t.Fatalf("second read error: %v", err)
	}
	if needsMigration2 {
		t.Fatal("migrated ciphertext should not need migration again")
	}
	if !bytes.Equal(result2, plain) {
		t.Fatalf("plaintext mismatch after migration: got %q", result2)
	}
}

func TestFallback_TamperedDataFailsBoth(t *testing.T) {
	key, _ := generateMasterKey()
	plain := []byte("tamper test")

	cipher, _ := encrypt(key, plain, []byte("aad"))
	cipher[nonceSize+2] ^= 0xFF

	_, _, err := decryptWithAADFallback(key, cipher, []byte("aad"))
	if err == nil {
		t.Fatal("tampered data should fail both AAD and nil-AAD paths")
	}
}
