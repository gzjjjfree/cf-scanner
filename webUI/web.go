package webUI

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/gorilla/websocket"
	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
	"github.com/schollz/progressbar/v3"
)

// 使用 go:embed 将前端文件打包进二进制
//
//go:embed web_frontend/dist
var staticFiles embed.FS

// 定义一个空的结构体，作为接口的载体
type WebLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

// 让 WebLogger 实现 WriteLog 方法
func (w WebLogger) WriteLog(msg string) {
	// WebSocket 广播推送
	clientsMu.Lock()
	defer clientsMu.Unlock()

	content := scanner.WSMessage{
		Type: "log",
		Data: msg,
	}
	payload, _ := json.Marshal(content)

	for client := range clients {
		// 直接发送原始字符串，包含 \r 或 \n
		client.WriteMessage(websocket.TextMessage, payload)
	}
}

func (w WebLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w WebLogger) GetColorCodes() bool {
	return false
}

// 方便外部调用的实例, 传参给外部让外部可以使用本包的方法
var BridgeLogger = WebLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

func Makeweb() {
	// 注册 API
	registerHandlers()

	// 注册静态资源
	distFS, _ := fs.Sub(staticFiles, "web_frontend/dist")
	http.Handle("/", http.FileServer(http.FS(distFS)))

	fmt.Println("API 接口已就绪: /api/status, /api/start")
	http.ListenAndServe(":8090", nil)
}

func OpenBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux": // Termux 属于 linux，但通常打不开安卓浏览器，需特殊处理
		err = exec.Command("termux-open", url).Run()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
	case "darwin":
		err = exec.Command("open", url).Run()
	}
	if err != nil {
		fmt.Println("打开浏览器出错！")
	}
}

// --- HTTP 处理器 ---
func registerHandlers() {
	http.HandleFunc("/ws", handleWebSocket) // WebSocket 入口
}

func runScannerLogic(ctx context.Context, conf scanner.ScanConfig) {
	// 告诉前端：扫描开始！
	broadcastStatus(scanner.Status.IsRunning)
	// 无论正常结束还是取消，最后都告诉前端：停下来。
	defer func() {
		scanner.StatusMutex.Lock()
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		broadcastStatus(scanner.Status.IsRunning)
	}()

	BridgeLogger.WriteLog("🚀 扫描任务启动...\n")

	BridgeLogger.WriteLog(fmt.Sprintf("[文件]: %s  [并发]: %d  [抽样]: %d/段  [目标]: %s\n", conf.FilePath, conf.NThreads, conf.TestNum, conf.SniDomain))
	BridgeLogger.WriteLog(fmt.Sprintf("[过滤]: 延迟 <%dms, 最低下载速度 >%v, 保留的数量 %v \n\n", conf.MinLatency, conf.MinSpeed, conf.FinalCount))

	// 扫描过程
	ipGroups, actualTaskCount := utils.ParseIP(conf.FilePath, conf.TestNum, BridgeLogger)
	if actualTaskCount <= 0 {
		BridgeLogger.WriteLog(fmt.Sprintln("读取 IP 文件出错，结束扫描！"))
		return
	}

	finalResults := scanner.RunScanPool(ctx, ipGroups, conf.NThreads, conf.SniDomain, int64(conf.MinLatency), actualTaskCount, BridgeLogger)

	BridgeLogger.WriteLog(fmt.Sprintf("\n--- 优选结果 Top %v 最后结果 %v---\n", conf.FinalCount*2, len(finalResults)))
	for i := 0; i < len(finalResults) && i < conf.FinalCount*2; i++ {
		BridgeLogger.WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency))
	}

	scanner.StatusMutex.Lock()
	if scanner.Status.WaitStop {
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		return
	}
	scanner.StatusMutex.Unlock()

	top := min(len(finalResults), max(scanner.Conf.FinalCount*2, 100))
	// 取前 outCount 名进行深度测速
	BridgeLogger.WriteLog(fmt.Sprintf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, conf.FinalCount))

	// 进行测速
	finalSorted := scanner.RunDeepTest(ctx, conf.FinalCount, conf.SniDomain, conf.MinSpeed, finalResults, BridgeLogger)

	// 结果已经存储在 finalSorted 切片中
	if len(finalSorted) > 0 {
		utils.SaveToCSV(conf.OutPrefix+".csv", finalSorted)
		utils.SaveToJSON(conf.OutPrefix+".json", finalSorted)

		err := utils.AppendToJSONFile(conf.JsonPath, finalSorted)
		if err != nil {
			BridgeLogger.WriteLog(fmt.Sprintf("保存文件失败: %v\n", err))
		} else {
			BridgeLogger.WriteLog(fmt.Sprintf("\n结果已追加至: %s\n", conf.JsonPath))
		}

		BridgeLogger.WriteLog(fmt.Sprintf("结果已保存至 %s.csv 和 %s.json\n", conf.OutPrefix, conf.OutPrefix))
	} else {
		BridgeLogger.WriteLog("\n本次未搜到优质 IP，保留旧的配置文件。")
	}

	BridgeLogger.WriteLog("\n✅ 优选后的 IP:")
	for i := 0; i < len(finalSorted); i++ {
		BridgeLogger.WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v  速度: %.2f MB/s\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs))
	}

	BridgeLogger.WriteLog("\n✅ 最终优选建议:")
	if len(finalSorted) > 0 {
		BridgeLogger.WriteLog(fmt.Sprintf("最佳 IP: [%s] | 预估带宽: %.2f MB/s\n", finalSorted[0].IP, finalSorted[0].DownloadMBs))
	}
	// 扫描结束
}
