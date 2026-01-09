package utils

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gzjjjfree/cf-scanner/scanner"
)

// saveToCSV 保存详细报告
func SaveToCSV(filename string, data []scanner.FinalResult) {
	// 1. 提取目录路径并创建 (例如 "result/result.csv")
	dir := filepath.Dir(filename)

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Println("创建目录失败:", err)
		return
	}

	// 2. 创建文件
	file, err := os.Create(filename)
	if err != nil {
		log.Println("创建文件失败:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"IP 地址", "延迟", "下载速度", "时间"})
	for _, r := range data {
		writer.Write([]string{
			r.IP,
			r.Latency,
			fmt.Sprintf("%.2f", r.DownloadMBs),
			r.CreatedAt.Format("2006-01-02 15:04:05"), // Go 的标准时间格式化写法
		})
	}
}

// saveToJSON 仅保存地址列表
func SaveToJSON(filename string, data []scanner.FinalResult) {
	file, _ := os.Create(filename)
	defer file.Close()

	// 只需要 JSON 里显示 address 字段，
	// FinalResult 里的其他字段在定义时加了 omitempty，且没有赋值时就会被隐藏
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	encoder.Encode(data)
}

// 追加保存到指定 JSON 文件
func AppendToJSONFile(path string, newResults []scanner.FinalResult) error {
	var existingData []map[string]interface{}

	// 尝试读取现有文件
	fileData, err := os.ReadFile(path)
	if err == nil && len(fileData) > 0 {
		// 如果文件存在且不为空，解析现有内容
		if err := json.Unmarshal(fileData, &existingData); err != nil {
			// 如果解析失败，说明原文件可能不是合法的 JSON 数组，记录警告
			fmt.Printf("警告: 原文件格式不兼容，将创建新数组: %v\n", err)
			existingData = []map[string]interface{}{}
		}
	}

	// 将新结果转换为 map 结构（为了只保留带 json 标签的字段）
	// 这样做可以确保忽略那些标记为 `json:"-"` 的字段
	for _, res := range newResults {
		// 我们通过这种方式只提取带 json 标签的字段
		item := map[string]interface{}{
			"address": res.IP,
		}

		// 可选：在这里做去重逻辑
		isDuplicate := false
		for _, existing := range existingData {
			if existing["address"] == res.IP {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			existingData = append(existingData, item)
		}
	}

	// 序列化回 JSON 数组（带缩进方便阅读）
	updatedJSON, err := json.MarshalIndent(existingData, "", "    ")
	if err != nil {
		return err
	}

	// 覆盖写入文件
	return os.WriteFile(path, updatedJSON, 0644)
}

// 从 CF 官网下载 IP 列表并保存
func DownloadFile(url string, filename string) error {
	//exePath, err := os.Executable()
	//fmt.Println("运行在: ", exePath)
	// 发起请求
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回状态码异常: %d", resp.StatusCode)
	}

	// 读取内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应体失败: %v", err)
	}

	// 写入文件
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	fmt.Println("已写入文件", filename)
	return nil
}

func GetDownloadURL(baseURL string, version string, repoName string) string {
	//baseURL := "https://github.com/gzjjjfree/v5-result/releases/download"
	//https://github.com/gzjjjfree/v5-result/releases/download/custom-build/v5-result-windows-amd64.exe
	var osName, arch, extension string

	// 判定操作系统
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
		extension = ".exe"
	case "darwin": // macOS
		osName = "macos"
		extension = ""
	case "linux":
		osName = "linux"
		extension = ""
	default:
		osName = "linux"
		extension = ""
	}

	// 判定架构
	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		arch = "amd64"
	}

	// 拼接符合你 Release 命名规则的字符串
	// 假设你的命名是：v5-result-windows-64.zip
	fileName := fmt.Sprintf("%s-%s-%s%s", repoName, osName, arch, extension)
	return fmt.Sprintf("%s/%s/%s", baseURL, version, fileName)
}
