package sys

import (
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// HostInfo 存储宿主机的静态信息
// 包含主机基本信息、CPU、内存和磁盘总量等非实时变动数据
type HostInfo struct {
	// 主机基本信息
	Hostname        string `json:"hostname"`         // 主机名
	OS              string `json:"os"`               // 操作系统 (e.g. linux, darwin)
	Platform        string `json:"platform"`         // 平台发行版 (e.g. ubuntu, centos)
	PlatformVersion string `json:"platform_version"` // 平台版本
	KernelVersion   string `json:"kernel_version"`   // 内核版本
	Arch            string `json:"arch"`             // 系统架构 (e.g. amd64, arm64)

	// CPU 信息
	CPUModelName string `json:"cpu_model_name"` // CPU 型号名称
	CPUCores     int    `json:"cpu_cores"`      // CPU 逻辑核心数

	// 内存信息
	TotalMemory uint64 `json:"total_memory"` // 物理内存总量 (Bytes)

	// 磁盘信息
	TotalDisk uint64 `json:"total_disk"` // 根分区磁盘总量 (Bytes)
}

// GetHostInfo 获取当前宿主机的静态配置信息
// 注意：此方法仅获取静态或总量信息，不包含实时的使用率数据
func GetHostInfo() (*HostInfo, error) {
	info := &HostInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// 1. 获取主机信息 (Hostname, Platform, KernelVersion 等)
	hInfo, err := host.Info()
	if err == nil {
		info.Hostname = hInfo.Hostname
		info.Platform = hInfo.Platform
		info.PlatformVersion = hInfo.PlatformVersion
		info.KernelVersion = hInfo.KernelVersion
	}

	// 2. 获取 CPU 信息
	// cpu.Info() 返回每个 CPU 的信息切片
	cInfos, err := cpu.Info()
	if err == nil && len(cInfos) > 0 {
		info.CPUModelName = cInfos[0].ModelName
	}
	// 获取逻辑核心数
	cores, err := cpu.Counts(true)
	if err == nil {
		info.CPUCores = cores
	}

	// 3. 获取内存总量
	mInfo, err := mem.VirtualMemory()
	if err == nil {
		info.TotalMemory = mInfo.Total
	}

	// 4. 获取磁盘总量 (根分区)
	// 在 Windows 上 "/" 可能对应当前驱动器根目录，在 Unix 上对应根分区
	dInfo, err := disk.Usage("/")
	if err == nil {
		info.TotalDisk = dInfo.Total
	}

	return info, nil
}
