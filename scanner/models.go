package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// 结构体定义，用于 JSON 和 CSV 导出
type FinalResult struct {
	IP          string    `json:"address"`
	Latency     string    `json:"-"` // 用于展示和 CSV 的字符串
	DownloadMBs float64   `json:"-"` // 下载速度
	RawLatency  int64     `json:"-"` // 内部排序用的数值 (ms)
	isSuccess   bool      `json:"-"`
	CreatedAt   time.Time `json:"-"` // 记录测试时间
}

// Logger 定义了扫描器需要的日志能力
// 只要任何类型实现了这个 WriteLog 方法，就可以传给扫描器
type Logger interface {
	WriteLog(string)
	GetTheme() progressbar.Theme // 获取进度条主题
	GetColorCodes() bool
}

// 定义全局变量
var (
	CancelScan  context.CancelFunc
	Conf        ScanConfig
	Status      ScanStatus
	StatusMutex sync.Mutex
)

type ScanStatus struct {
	IsRunning bool `json:"is_running"`
	WaitStop  bool
}

// 定义发送给前端的消息结构
type WSMessage struct {
	Type string `json:"type"` // "log" 或 "status"
	Data any    `json:"data"` // 日志内容 或 状态对象
}

type ScanConfig struct {
	NThreads   int     `json:"nthreads"`    // 并发协程数
	MinLatency int64   `json:"min_latency"` // 最小延迟
	FinalCount int     `json:"final_count"` // 最终结果保留的数
	MinSpeed   float64 `json:"min_speed"`   // 最小下载速度
	TestNum    int     `json:"test_num"`    // 每段 IP 抽样数
	FilePath   string  `json:"file_path"`   // IP 来源文件路径
	SniDomain  string  `json:"sni_domain"`  // 测试的地址
	OutPrefix  string  `json:"out_prefix"`  // 输出文件的前缀
	JsonPath   string  `json:"json_path"`   // 追加文件的路径
	AppendMode bool    `json:"append_mode"` // 是否启用追加
	ShowVer    bool    `json:"show_ver"`    // 是否显示版本
	ShowWeb    bool    `json:"show_web"`    // 是否启用 web GUI
	ShouldRun  bool    `json:"should_run"`  // 运行扫描
	DownloadV5 bool    // 是否下载 v5-result
	Wsconnet   bool    `json:"wsconnet"` // 检测 WS 连接可用性
}

// 检查扫描参数，防止不合理设置
func (conf *ScanConfig) Check() {
	if conf.NThreads <= 0 || conf.NThreads > 500 {
		conf.NThreads = 100 // 给予默认值
	}

	if conf.MinLatency <= 10 || conf.MinLatency > 2000 {
		conf.MinLatency = 200 // 给予默认值
	}

	if conf.FinalCount <= 1 || conf.FinalCount > 1000 {
		conf.FinalCount = 10 // 给予默认值
	}

	if conf.MinSpeed <= 0.1 || conf.MinSpeed > 30 {
		conf.MinSpeed = 5 // 给予默认值
	}

	if conf.TestNum <= 1 || conf.TestNum > 5000 {
		conf.TestNum = 500 // 给予默认值
	}

	if conf.FilePath == "" {
		conf.FilePath = "ip.txt" // 给予默认值
	}

	if !IsValidURL(conf.SniDomain) {
		conf.SniDomain = "speed.cloudflare.com/__down?bytes=100000000" // 给予默认值
	}

	if conf.OutPrefix == "" {
		conf.OutPrefix = "result/result" // 给予默认值
	}

	if conf.JsonPath == "" {
		conf.JsonPath = "okresult.json" // 给予默认值
	}
}

func IsValidURL(str string) bool {
	u, err := url.Parse(str)
	// 解析出错、协议为空、或主机名为空，都视为非法
	if err != nil || u.Scheme == "" || u.Host == "" {
		u, err = url.Parse("https://" + str)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return false
		}
	}
	return true
}

type IPAddr struct {
	Address string `json:"address"`
}

func CheckWSConnections(domain string, okPath string, resultPath string) error {
	// 1. 设置默认值逻辑
	if okPath == "" {
		okPath = "result/result1.json"
	}
	if resultPath == "" {
		resultPath = "result/result.json"
	}

	// 2. 读取并合并地址
	ipSet := make(map[string]struct{}) // 用于去重

	dir, _ := os.Getwd()
	fmt.Printf("当前工作目录: %s\n", dir)

	paths := []string{okPath, resultPath}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("跳过无法读取的文件: %s\n", path)
			continue
		}

		var entries []IPAddr
		if err := json.Unmarshal(data, &entries); err != nil {
			fmt.Printf("解析文件 %s 失败: %v\n", path, err)
			continue
		}

		for _, entry := range entries {
			if entry.Address != "" {
				ipSet[entry.Address] = struct{}{}
			}
		}
	}

	if len(ipSet) == 0 {
		return fmt.Errorf("没有从指定文件中读取到任何有效地址")
	}

	fmt.Printf("共加载 %d 个唯一 IP，开始检测域名 [%s] 的 WS 可用性...\n", len(ipSet), domain)

	// 3. 执行检测
	var invalidIPs []IPAddr
	var validIPs []IPAddr
	for ip := range ipSet {
		if checkWSAvailability(ip, domain) {
			fmt.Printf("[√] IP %s 可用\n", ip)
			validIPs = append(validIPs, IPAddr{Address: ip})
		} else {
			fmt.Printf("[X] IP %s 不可用 (403 或连接失败)\n", ip)
			invalidIPs = append(invalidIPs, IPAddr{Address: ip}) // 记录失败者
		}
	}

	// 4. (可选) 将结果保存到新文件，防止下次重复检测
	if len(validIPs) > 0 {
		outputData, _ := json.MarshalIndent(validIPs, "", "    ")
		os.WriteFile("ws_verified_results.json", outputData, 0644)
		fmt.Printf("检测完成，已将 %d 个可用 IP 保存至 ws_verified_results.json\n", len(validIPs))
	}

	// 4. 将检测失败（不可用）的 IP 追加到 notwork.json
	if len(invalidIPs) > 0 {
		blacklistPath := "notwork.json"
		var currentBlacklist []IPAddr

		// 1. 尝试读取现有的黑名单
		if data, err := os.ReadFile(blacklistPath); err == nil {
			json.Unmarshal(data, &currentBlacklist)
		}

		// 2. 合并新失效的 IP 并去重
		fullMap := make(map[string]struct{})
		for _, item := range currentBlacklist {
			fullMap[item.Address] = struct{}{}
		}
		for _, item := range invalidIPs {
			fullMap[item.Address] = struct{}{}
		}

		// 3. 转回切片结构
		var updatedBlacklist []IPAddr
		for ip := range fullMap {
			updatedBlacklist = append(updatedBlacklist, IPAddr{Address: ip})
		}

		// 4. 重新写回文件
		outputData, _ := json.MarshalIndent(updatedBlacklist, "", "    ")
		err := os.WriteFile(blacklistPath, outputData, 0644)

		if err == nil {
			fmt.Printf("已将 %d 个失效 IP 追加至 %s (总计 %d 个黑名单)\n",
				len(invalidIPs), blacklistPath, len(updatedBlacklist))
		} else {
			fmt.Printf("保存黑名单失败: %v\n", err)
		}
	}

	return nil
}
