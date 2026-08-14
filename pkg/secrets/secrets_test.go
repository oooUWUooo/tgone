package secrets

import (
	"os"
	"testing"
)

func TestNewSecretStore(t *testing.T) {
	store, err := NewSecretStore("test_secrets.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.secrets == nil {
		t.Fatal("Expected secrets map to be initialized")
	}

	// Cleanup test file
	os.Remove("test_secrets.yaml")
}

func TestGetAndSet(t *testing.T) {
	store, err := NewSecretStore("test_secrets_getset.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_getset.yaml")

	key := "test.key"
	value := "test_value"

	store.Set(key, value)

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}
	if got != value {
		t.Errorf("Expected %s, got %s", value, got)
	}
}

func TestGetNotFound(t *testing.T) {
	store, err := NewSecretStore("test_secrets_notfound.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_notfound.yaml")

	_, err = store.Get("nonexistent.key")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestDelete(t *testing.T) {
	store, err := NewSecretStore("test_secrets_delete.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_delete.yaml")

	key := "test.key"
	value := "test_value"

	store.Set(key, value)
	store.Delete(key)

	_, err = store.Get(key)
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestGetOrEnv(t *testing.T) {
	store, err := NewSecretStore("test_secrets_env.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_env.yaml")

	// Set environment variable
	envKey := "TEST_ENV_VAR"
	envValue := "env_value"
	os.Setenv(envKey, envValue)
	defer os.Unsetenv(envKey)

	// Get from env when secret doesn't exist
	got := store.GetOrEnv("nonexistent.key", envKey)
	if got != envValue {
		t.Errorf("Expected %s from env, got %s", envValue, got)
	}

	// Set secret and verify it takes precedence
	secretValue := "secret_value"
	store.Set("test.key", secretValue)
	got = store.GetOrEnv("test.key", envKey)
	if got != secretValue {
		t.Errorf("Expected %s from secret, got %s", secretValue, got)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	store, err := NewSecretStore("test_secrets_encrypt.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_encrypt.yaml")

	// Set encryption key
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i)
	}
	store.encryptionKey = testKey

	plaintext := "Hello, World!"
	ciphertext, err := store.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := store.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}

func TestSaveAndLoad(t *testing.T) {
	configPath := "test_secrets_saveload.yaml"
	store, err := NewSecretStore(configPath)
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove(configPath)

	// Set some values
	store.Set("telegram.bot_token", "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11")
	store.Set("database.sqlite_path", "./test.db")

	// Save
	err = store.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Create new store and load
	store2, err := NewSecretStore(configPath)
	if err != nil {
		t.Fatalf("Failed to load secret store: %v", err)
	}

	token, err := store2.Get("telegram.bot_token")
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}
	if token != "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11" {
		t.Errorf("Token mismatch")
	}
}

func TestConcurrentAccess(t *testing.T) {
	store, err := NewSecretStore("test_secrets_concurrent.yaml")
	if err != nil {
		t.Fatalf("Failed to create secret store: %v", err)
	}
	defer os.Remove("test_secrets_concurrent.yaml")

	done := make(chan bool)

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a' + id))
				store.Set(key, "value")
				store.Get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
