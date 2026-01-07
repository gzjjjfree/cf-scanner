package main

import (
	"context"
	"embed"
	"fmt"

	"github.com/gzjjjfree/cf-scanner/cmd"
	"github.com/gzjjjfree/cf-scanner/scanner"
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
		Title: fmt.Sprint("CF-Scanner Wails版       版本: ", cmd.Version),
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
	ctx context.Context
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

// StartScan 模拟你的 cf-scanner 扫描过程
func (a *SimpleApp) StartScan(cmd WSMessage) {	
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
			scanner.Status.IsRunning = false
			broadcastStatus(a.ctx, false)
		}

		// 创建一个真正的 map[string]string
		cleanParams := make(map[string]string)
		for k, v := range rawParams {
			// 将 any 转换为 string（处理字符串、数字、布尔等）
			cleanParams[k] = fmt.Sprintf("%v", v)
		}		
		// 你可以从 cmd.Type 中提取前端传来的参数
		go startScanWorkflow(a, cleanParams)
		
	case "stop":
		scanner.Status.WaitStop = true
		a.stopScan()
	}
}

func (a *SimpleApp) stopScan() {
	scanner.StatusMutex.Lock()
	defer scanner.StatusMutex.Unlock()
	if scanner.CancelScan != nil {
		scanner.CancelScan() // 触发取消信号
		runtime.EventsEmit(a.ctx, "scan_log", "\n🛑 正在通过 WebSocket 指令停止任务...\n")
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
