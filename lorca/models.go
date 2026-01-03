package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/schollz/progressbar/v3"
)

// 定义发送给前端的消息结构
type WSMessage struct {
	Type string `json:"type"` // "log" 或 "status"
	Data any    `json:"data"` // 日志内容 或 状态对象
}

// ScanStatus 定义了要返回给前端的 JSON 结构
type scanStatus struct {
	IsRunning bool `json:"is_running"`
	waitStop  bool
}

// 使用互斥锁确保并发安全，防止多线程同时写状态导致程序崩溃
var (
	status      scanStatus
	statusMutex sync.Mutex
)

type scanConfig struct {
	Threads    int     `json:"threads"`
	MinLatency int     `json:"min_latency"`
	FinalCount int     `json:"final_count"`
	MinSpeed   float64 `json:"min_speed"`
	TestNum    int     `json:"test_num"`
}

// 定义一个空的结构体，作为接口的载体
type LorcaLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

// 让 WebLogger 实现 WriteLog 方法
func (w LorcaLogger) WriteLog(msg string) {
	WriteLog(msg)
}

func (w LorcaLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w LorcaLogger) GetColorCodes() bool {
	return false
}

var BridgeLogger = LorcaLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

// 捕获日志并更新到状态中
func WriteLog(msg string) {
	fmt.Print(msg)

	content := WSMessage{
		Type: "log",
		Data: msg,
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		fmt.Printf("JSON 序列化失败: %v\n", err)
		return
	}

	ui.Eval(fmt.Sprintf(`window.handleScanUpdate(%s)`, string(jsonData)))
}
