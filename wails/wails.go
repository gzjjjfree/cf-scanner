package main

import (
	"context"
	"embed"
	"fmt"
	"sync"

	"github.com/schollz/progressbar/v3"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 创建一个简单的 App 结构体
	wailsapp := NewApp()

	err := wails.Run(&options.App{
		Title: "CF-Scanner",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: wailsapp.Startup,
		Bind: []interface{}{
			wailsapp,
		},
	})

	if err != nil {
		fmt.Println("Error:", err)
	}
}

// SimpleApp 给前端调用的结构体
type SimpleApp struct {
	ctx    context.Context
	cancel context.CancelFunc // 存放取消函数
	mu     sync.Mutex         // 增加锁，防止并发操作 cancel 导致崩溃
}

func NewApp() *SimpleApp {
	return &SimpleApp{}
}

// Startup 在程序启动时自动保存 context
func (a *SimpleApp) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *SimpleApp) WriteLog(msg string) {
	runtime.EventsEmit(a.ctx, "scan_log", msg)
}

func (s *SimpleApp) Greet(name string) string {
	return fmt.Sprintf("Hello %s, 桌面模式启动成功!", name)
}

// ScanStatus 定义了要返回给前端的 JSON 结构
type ScanStatus struct {
	IsRunning bool `json:"is_running"`
	waitStop  bool
}

// 使用互斥锁确保并发安全，防止多线程同时写状态导致程序崩溃
var (
	status      ScanStatus
	statusMutex sync.Mutex
)

// StartScan 模拟你的 cf-scanner 扫描过程
func (a *SimpleApp) StartScan(cmd WSMessage) {
	a.mu.Lock()
	// 如果已经在运行，先停止旧的（可选，视逻辑而定）
	if a.cancel != nil {
		a.cancel()
	}
	// 根据 Type 分发逻辑

	// 每次开始前，创建一个可取消的 context
	appCtx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.mu.Unlock()

	switch cmd.Type {
	case "start":
		// 首先断言 cmd.Data 是一个 map
		rawParams, ok := cmd.Data.(map[string]any)
		if !ok {
			content := WSMessage{
				Type: "log",
				Data: "❌ 错误：无效的参数格式\n",
			}
			runtime.EventsEmit(a.ctx, "scan_log", content)
			status.IsRunning = false
			broadcastStatus(appCtx, false)
		}

		// 创建一个真正的 map[string]string
		cleanParams := make(map[string]string)
		for k, v := range rawParams {
			// 将 any 转换为 string（处理字符串、数字、布尔等）
			cleanParams[k] = fmt.Sprintf("%v", v)
		}
		// 你可以从 cmd.Type 中提取前端传来的参数
		go func() {
			startScanWorkflow(appCtx, cleanParams)

			// 任务彻底结束后，清理 cancel 防止内存泄漏
			a.mu.Lock()
			a.cancel = nil
			a.mu.Unlock()
		}()
	case "stop":
		a.StopScan()
		status.waitStop = true
		content := WSMessage{
			Type: "log",
			Data: "\n🛑 正在通过 WebSocket 指令停止任务...\n",
		}
		runtime.EventsEmit(a.ctx, "scan_log", content)
	}
}

func (a *SimpleApp) StopScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel() // 触发取消信号
		fmt.Println("🛑 已下达停止指令")
	}
}

// 在 App 结构体中添加这个 Logger
func (a *SimpleApp) GetLogger() *WailsLogger {
	return &WailsLogger{Ctx: a.ctx}
}

func GetScanning() bool {
	return true
}

// 定义一个空的结构体，作为接口的载体
type WailsLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

// 让 WebLogger 实现 WriteLog 方法
func (w WailsLogger) WriteLog(msg string) {
	fmt.Print(msg)

	// 将日志推送到前端
	content := WSMessage{
		Type: "log",
		Data: msg,
	}
	runtime.EventsEmit(w.Ctx, "scan_log", content)
}

func (w WailsLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w WailsLogger) GetColorCodes() bool {
	return false
}
