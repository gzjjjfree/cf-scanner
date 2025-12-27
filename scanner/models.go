package scanner

import "time"

// 结构体定义，用于 JSON 和 CSV 导出
type FinalResult struct {
	IP          string    `json:"address"`
	Latency     string    `json:"-"` // 用于展示和 CSV 的字符串
	DownloadMBs float64   `json:"-"` // 下载速度
	RawLatency  int64     `json:"-"` // 内部排序用的数值 (ms)
	isSuccess   bool      `json:"-"`
	CreatedAt   time.Time `json:"-"` // 新增：记录测试时间
}
