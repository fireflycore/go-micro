package sys

import (
	"testing"
)

func TestGetHostInfo(t *testing.T) {
	info, err := GetHostInfo()
	if err != nil {
		t.Fatalf("GetHostInfo() failed: %v", err)
	}

	if info == nil {
		t.Fatal("GetHostInfo() returned nil info")
	}

	// 验证基本信息是否存在
	if info.OS == "" {
		t.Error("HostInfo.OS is empty")
	}
	if info.Arch == "" {
		t.Error("HostInfo.Arch is empty")
	}
	if info.Hostname == "" {
		t.Log("Warning: HostInfo.Hostname is empty") // 某些环境可能为空，只记录日志
	}
	
	// 验证硬件信息是否合理
	if info.CPUCores <= 0 {
		t.Errorf("HostInfo.CPUCores should be positive, got %d", info.CPUCores)
	}
	if info.TotalMemory <= 0 {
		t.Errorf("HostInfo.TotalMemory should be positive, got %d", info.TotalMemory)
	}
	if info.TotalDisk <= 0 {
		t.Errorf("HostInfo.TotalDisk should be positive, got %d", info.TotalDisk)
	}

	// 打印信息以便人工确认
	t.Logf("Host Info Retrieved: %+v", info)
}
