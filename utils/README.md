# Utils

`utils` 包提供了一系列通用的工具函数，涵盖了网络、切片、文件、结构体、进程管理、树结构操作及版本控制等功能。

## 功能列表

### 1. Network (网络)
提供网络相关的实用函数。

- **GetInternalNetworkIp**
  - 签名: `func GetInternalNetworkIp() string`
  - 描述: 获取本机在局域网中的 IP 地址。通过建立 UDP 伪连接（不发送数据）来自动选择合适的路由接口 IP。

### 2. Slice (切片)
提供泛型切片操作工具。

- **DiffSlices**
  - 签名: `func DiffSlices[T comparable](old, new []T) (add, remove []T)`
  - 描述: 比较两个切片的差异，返回新增和删除的元素列表。
- **FilterSlice**
  - 签名: `func FilterSlice[T any](array []T, fn func(index int, item T) bool) []T`
  - 描述: 根据过滤函数 `fn` 筛选切片中的元素，返回保留下来的元素组成的新切片。

### 3. File (文件)
提供文件读写辅助功能。

- **ReadRemote**
  - 签名: `func ReadRemote(url string) ([]byte, error)`
  - 描述: 读取远程 URL 的文件内容。
- **WriteLocal**
  - 签名: `func WriteLocal(path string, bytes []byte) error`
  - 描述: 将字节内容写入本地文件，如果目录不存在会自动创建。

### 4. Struct (结构体)
提供结构体转换与比较工具。

- **StructConvert**
  - 签名: `func StructConvert[S any, T any](source *S, target *T)`
  - 描述: 通过 JSON 序列化/反序列化实现结构体之间的转换。注意：性能较低，仅适用于非关键路径。
- **DiffStruct**
  - 签名: `func DiffStruct[O any, N any](old O, new N, ignore []string) map[string]interface{}`
  - 描述: 比较两个结构体，返回发生变更的字段名及其新值。支持忽略指定字段。

### 5. Process (进程)
提供进程信号处理工具。

- **Watcher**
  - 签名: `func Watcher(handle func())`
  - 描述: 阻塞监听系统信号（SIGTERM, SIGQUIT, SIGINT），在接收到信号时执行回调函数 `handle`，常用于优雅关闭服务。

### 6. Tree (LTree)
提供标签树（Label Tree）路径操作工具。

- **LTree** 结构体方法:
  - `GetLTreeDepth(path string) int`: 计算路径深度。
  - `BuildLTreePath(parentPath, currentValue string) string`: 构建路径。
  - `ExtractLastSegment(path string) string`: 提取路径最后一段。
  - `IsDescendantPath(descendantPath, ancestorPath string) bool`: 判断是否为后代路径。
  - `ReplacePathPrefix(oldPath, newPath, fullPath string) string`: 替换路径前缀。
  - `ReplaceCurrentPath(oldPath, currentPath string) string`: 替换当前路径节点。

### 7. Version (版本)
提供语义化版本管理工具。

- **Version** 结构体: 包含 `Major`, `Minor`, `Patch`。
- **NewVersion**
  - 签名: `func NewVersion(verStr string) (*Version, error)`
  - 描述: 解析版本字符串（如 "v1.0.1"）。
- **相关方法**:
  - `String()`: 格式化为字符串。
  - `Increment()`: 版本号自增逻辑（Patch -> Minor -> Major）。
  - `Compare(other *Version) int`: 版本比较。
