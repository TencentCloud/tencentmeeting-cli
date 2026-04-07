package common

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// ────────────────────────────────────────────────
// generateRandomMachineID
// ────────────────────────────────────────────────

// TestGenerateRandomMachineID_Length 验证生成的 ID 长度为 48
func TestGenerateRandomMachineID_Length(t *testing.T) {
	id := generateRandomMachineID()
	if len(id) != 48 {
		t.Errorf("期望长度 48，实际长度 %d，id=%q", len(id), id)
	}
}

// TestGenerateRandomMachineID_Charset 验证生成的 ID 只包含字母和数字
func TestGenerateRandomMachineID_Charset(t *testing.T) {
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	for i := 0; i < 100; i++ {
		id := generateRandomMachineID()
		if !re.MatchString(id) {
			t.Errorf("ID 包含非法字符: %q", id)
		}
	}
}

// TestGenerateRandomMachineID_Uniqueness 多次调用结果不应完全相同
func TestGenerateRandomMachineID_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		id := generateRandomMachineID()
		seen[id] = struct{}{}
	}
	if len(seen) == 1 {
		t.Error("10 次调用全部返回相同 ID，随机性异常")
	}
}

// ────────────────────────────────────────────────
// readMachineIDFromFile / writeMachineIDToFile
// ────────────────────────────────────────────────

// TestReadMachineIDFromFile_NotExist 文件不存在时应返回错误
func TestReadMachineIDFromFile_NotExist(t *testing.T) {
	id, err := readMachineIDFromFile(filepath.Join(t.TempDir(), "not_exist"))
	if err == nil {
		t.Errorf("期望返回错误，实际返回 id=%q", id)
	}
}

// TestReadMachineIDFromFile_TrimSpace 读取时应去除首尾空白
func TestReadMachineIDFromFile_TrimSpace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "machine_id")
	if err := os.WriteFile(path, []byte("  abc123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	id, err := readMachineIDFromFile(path)
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if id != "abc123" {
		t.Errorf("期望 %q，实际 %q", "abc123", id)
	}
}

// TestWriteMachineIDToFile_Success 正常写入并可读回
func TestWriteMachineIDToFile_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "machine_id")
	want := "testID_12345"
	if err := writeMachineIDToFile(path, want); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	got, err := readMachineIDFromFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if got != want {
		t.Errorf("期望 %q，实际 %q", want, got)
	}
}

// TestWriteMachineIDToFile_CreateDir 目录不存在时应自动创建
func TestWriteMachineIDToFile_CreateDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "machine_id")
	if err := writeMachineIDToFile(path, "id_abc"); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("文件不存在: %v", err)
	}
}

// TestWriteMachineIDToFile_ExistFails 文件已存在时应返回错误（O_EXCL 语义）
func TestWriteMachineIDToFile_ExistFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "machine_id")
	// 第一次写入成功
	if err := writeMachineIDToFile(path, "first"); err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}
	// 第二次写入应失败
	if err := writeMachineIDToFile(path, "second"); err == nil {
		t.Error("期望第二次写入返回错误（文件已存在），实际返回 nil")
	}
	// 文件内容应仍为第一次写入的值
	got, _ := readMachineIDFromFile(path)
	if got != "first" {
		t.Errorf("文件内容被覆盖，期望 %q，实际 %q", "first", got)
	}
}

// TestWriteMachineIDToFile_ConcurrentOnlyFirstSucceeds 并发写入只有第一个成功
func TestWriteMachineIDToFile_ConcurrentOnlyFirstSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "machine_id")
	const goroutines = 20

	var (
		wg           sync.WaitGroup
		successCount int
		mu           sync.Mutex
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			err := writeMachineIDToFile(path, generateRandomMachineID())
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("期望只有 1 个 goroutine 写入成功，实际 %d 个成功", successCount)
	}
}

// ────────────────────────────────────────────────
// GetSystemInfo
// ────────────────────────────────────────────────

// TestGetSystemInfo_ReturnNotNil 返回值不为 nil
func TestGetSystemInfo_ReturnNotNil(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", t.TempDir())
	info := GetSystemInfo()
	if info == nil {
		t.Fatal("GetSystemInfo 返回 nil")
	}
}

// TestGetSystemInfo_MachineIDLength machineID 长度为 48
func TestGetSystemInfo_MachineIDLength(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", t.TempDir())
	info := GetSystemInfo()
	if len(info.MachineID) != 48 {
		t.Errorf("期望 MachineID 长度 48，实际 %d，id=%q", len(info.MachineID), info.MachineID)
	}
}

// TestGetSystemInfo_MachineIDCached 第二次调用应返回相同的 machineID（读缓存）
func TestGetSystemInfo_MachineIDCached(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", t.TempDir())
	first := GetSystemInfo()
	second := GetSystemInfo()
	if first.MachineID != second.MachineID {
		t.Errorf("两次调用 machineID 不一致: %q vs %q", first.MachineID, second.MachineID)
	}
}

// TestGetSystemInfo_OSNotEmpty OS 字段不为空
func TestGetSystemInfo_OSNotEmpty(t *testing.T) {
	t.Setenv("TMEET_CLI_CONFIG_DIR", t.TempDir())
	info := GetSystemInfo()
	if strings.TrimSpace(info.OS) == "" {
		t.Error("OS 字段不应为空")
	}
}

// TestGetSystemInfo_UseExistingCacheFile 若缓存文件已存在，应直接读取其内容
func TestGetSystemInfo_UseExistingCacheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMEET_CLI_CONFIG_DIR", dir)

	// 预写一个固定 ID 到缓存文件
	fixedID := strings.Repeat("x", 48)
	cacheFile := filepath.Join(dir, machineIDFile)
	if err := os.WriteFile(cacheFile, []byte(fixedID), 0600); err != nil {
		t.Fatal(err)
	}

	info := GetSystemInfo()
	if info.MachineID != fixedID {
		t.Errorf("期望读取缓存 ID %q，实际 %q", fixedID, info.MachineID)
	}
}

// ────────────────────────────────────────────────
// readOSReleasePrettyName（仅在 Linux 上有意义，其他平台跳过）
// ────────────────────────────────────────────────

// TestReadOSReleasePrettyName_ParsesCorrectly 验证能正确解析 PRETTY_NAME 字段
func TestReadOSReleasePrettyName_ParsesCorrectly(t *testing.T) {
	content := `ID=ubuntu
VERSION_ID="22.04"
PRETTY_NAME="Ubuntu 22.04.3 LTS"
HOME_URL="https://www.ubuntu.com/"
`
	// 写入临时文件，通过直接调用解析逻辑验证
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 直接解析文件内容（复用内部逻辑）
	data, _ := os.ReadFile(path)
	got := parsePrettyName(string(data))
	if got != "Ubuntu 22.04.3 LTS" {
		t.Errorf("期望 %q，实际 %q", "Ubuntu 22.04.3 LTS", got)
	}
}
