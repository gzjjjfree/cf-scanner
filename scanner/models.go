package scanner

import (
	"context"
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

// 全局定义取消函数
var CancelScan context.CancelFunc

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

	if conf.SniDomain == "" {
		conf.SniDomain = "speed.cloudflare.com/__down?bytes=100000000" // 给予默认值
	}

	if conf.OutPrefix == "" {
		conf.OutPrefix = "result" // 给予默认值
	}

	if conf.JsonPath == "" {
		conf.JsonPath = "okresult.json" // 给予默认值
	}
}
