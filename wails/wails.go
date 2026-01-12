package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/gzjjjfree/cf-scanner/cmd"
	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
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
		Title: fmt.Sprint("CF-Scanner    Wails: ", cmd.Version),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: wailsapp.Startup,
		Bind: []any{
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
			broadcastStatus(a.ctx, "is_scanning", scanner.Status.IsRunning)
		}

		scanner.StatusMutex.Lock()
		if scanner.CancelScan != nil {
			fmt.Println("有任务在执行, 取消旧任务...")
			scanner.CancelScan()
		}
		var appCtx context.Context
		appCtx, scanner.CancelScan = context.WithCancel(a.ctx)
		BridgeLogger.Ctx = appCtx
		scanner.StatusMutex.Unlock()

		// 创建一个真正的 map[string]string
		cleanParams := make(map[string]string)
		for k, v := range rawParams {
			// 将 any 转换为 string（处理字符串、数字、布尔等）
			cleanParams[k] = fmt.Sprintf("%v", v)
		}
		// 你可以从 cmd.Type 中提取前端传来的参数
		go startScanWorkflow(appCtx, cleanParams)

	case "stop":
		scanner.Status.WaitStop = true
		a.stopScan()
	case "download":
		//os.Setenv("HTTP_PROXY", "http://127.0.0.1:10814")
		//os.Setenv("HTTPS_PROXY", "http://127.0.0.1:10814")
		//fmt.Println("https_proxy", os.Getenv("HTTPS_PROXY"))

		targetURL := utils.GetDownloadURL("https://github.com/gzjjjfree/v5-result/releases/download", "custom-build", "v5-result") // 这里可以换成你动态获取的最新版本号
		fileName := path.Base(targetURL)

		// 1. 获取可执行文件所在的目录
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath) // 获取 /home/.../build/bin 这个目录

		// 2. 拼接完整的目标路径
		fullPath := filepath.Join(exeDir, fileName)

		scanner.StatusMutex.Lock()
		if scanner.CancelScan != nil {
			fmt.Println("有任务在执行, 取消旧任务...")
			scanner.CancelScan()
		}
		var appCtx context.Context
		appCtx, scanner.CancelScan = context.WithCancel(a.ctx)
		BridgeLogger.Ctx = appCtx
		scanner.StatusMutex.Unlock()

		BridgeLogger.WriteLog("正在下载 v5-result\n")
		BridgeLogger.WriteLog(fmt.Sprintln("下载目录在: ", exeDir))

		go func(ctx context.Context) {
			broadcastStatus(ctx, "is_downloading", true)
			err := utils.DownloadFile(ctx, targetURL, fullPath, BridgeLogger)
			os.Setenv("HTTP_PROXY", "")
			os.Setenv("HTTPS_PROXY", "")
			if err == nil {
				runtime.EventsEmit(ctx, "scan_log", WSMessage{Type: "directions", Data: utils.DownloadMsg})				
			}
			broadcastStatus(ctx, "is_downloading", false)
		}(appCtx)
	}
}

func (a *SimpleApp) stopScan() {
	scanner.StatusMutex.Lock()
	defer scanner.StatusMutex.Unlock()
	if scanner.CancelScan != nil {
		scanner.CancelScan() // 触发取消信号
		scanner.CancelScan = nil
		runtime.EventsEmit(a.ctx, "scan_log", WSMessage{Type: "log", Data: "\n🛑 正在通过 CancelScan() 指令停止任务...\n"})
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
