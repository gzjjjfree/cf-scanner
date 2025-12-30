package gui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
	"github.com/schollz/progressbar/v3"
)

// 使用 go:embed 将前端文件打包进二进制
//
//go:embed frontend/dist
var staticFiles embed.FS

// ScanStatus 定义了要返回给前端的 JSON 结构
type ScanStatus struct {
	IsRunning bool   `json:"is_running"`
	Progress  int    `json:"progress"`
	LastLog   string `json:"last_log"`
}

// 使用互斥锁确保并发安全，防止多线程同时写状态导致程序崩溃
var (
	status      ScanStatus
	statusMutex sync.Mutex
)

type ScanConfig struct {
	Threads    int     `json:"threads"`
	MinLatency int     `json:"min_latency"`
	FinalCount int     `json:"final_count"`
	MinSpeed   float64 `json:"min_speed"`
	TestNum    int     `json:"test_num"`
}

// 更新状态的辅助函数
func updateStatus(fn func(*ScanStatus)) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	fn(&status)
}

// 捕获日志并更新到状态中
func WriteLog(msg string) {
	// 1. 同时在终端显示（保持原有 powershell 输出）
	fmt.Print(msg)

	// 2. 更新到全局状态，供前端轮询
	updateStatus(func(s *ScanStatus) {
		// 模拟终端行为：处理回车符 \r
		if strings.Contains(msg, "\r") {
			// 找到最后一个换行符的位置
			parts := strings.Split(s.LastLog, "\n")
			// 只保留最后一行之前的全部内容 + 最新的进度条行
			if len(parts) > 0 {
				newLastLine := strings.Split(msg, "\r")
				parts[len(parts)-1] = newLastLine[len(newLastLine)-1]
				s.LastLog = strings.Join(parts, "\n")
			}
		} else {
			s.LastLog += msg
		}
	})
}

// 定义一个空的结构体，作为接口的载体
type WebLogger struct {
	Theme progressbar.Theme
}

// 让 WebLogger 实现 WriteLog 方法
func (w WebLogger) WriteLog(msg string) {
	// 调用你之前写好的那个全局 WriteLog 函数
	WriteLog(msg)
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

func Makeweb(fPath string, sniDomain string) {
	// 注册 API
	registerHandlers(fPath, sniDomain)

	// 注册静态资源 (使用你之前的 embed)
	distFS, _ := fs.Sub(staticFiles, "frontend/dist")
	http.Handle("/", http.FileServer(http.FS(distFS)))

	fmt.Println("API 接口已就绪: /api/status, /api/start")
	http.ListenAndServe(":8080", nil)
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

func registerHandlers(fPath string, sniDomain string) {
	// 1. 获取当前状态
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		statusMutex.Lock()

		// 拷贝一份数据出来，立即解锁，然后再进行 JSON 编码
		// 这样编码的时间就不会占用锁
		current := status
		statusMutex.Unlock()

		json.NewEncoder(w).Encode(current)
	})

	// 2. 开始扫描
	http.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		statusMutex.Lock()
		if status.IsRunning {
			statusMutex.Unlock()
			http.Error(w, "Task already running", http.StatusConflict)
			return
		}
		status.IsRunning = true
		status.Progress = 0
		statusMutex.Unlock()

		// 打印请求的所有信息
		fmt.Printf("收到请求: Method=%s, ContentLength=%d\n", r.Method, r.ContentLength)

		// 如果 ContentLength <= 0，说明前端真的没传数据过来
		if r.ContentLength <= 0 {
			fmt.Println("警告: 这是一个空请求体!")
		}

		var config ScanConfig
		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			fmt.Printf("JSON 解析失败: %v\n", err)
			http.Error(w, "参数错误", 400)
			return
		}
		// 异步启动你的扫描逻辑，不要阻塞 Web 线程
		go runScannerLogic(config, fPath, sniDomain)

		w.Write([]byte("Started"))
	})
}

func runScannerLogic(conf ScanConfig, fPath string, sniDomain string) {
	WriteLog("🚀 扫描任务启动...\n")

	WriteLog(fmt.Sprintf("   [文件]: %s  [并发]: %d  [抽样]: %d/段  [目标]: %s\n", fPath, conf.Threads, conf.TestNum, sniDomain))
	WriteLog(fmt.Sprintf("   [过滤]: 延迟 <%dms, 最低下载速度 >%v, 保留的数量 %v \n\n", conf.MinLatency, conf.MinSpeed, conf.FinalCount))

	// 扫描过程
	ipGroups, actualTaskCount := utils.ParseIP(fPath, conf.TestNum, BridgeLogger)

	finalResults := scanner.RunScanPool(ipGroups, conf.Threads, sniDomain, int64(conf.MinLatency), actualTaskCount, BridgeLogger)

	WriteLog(fmt.Sprintf("\n--- 优选结果 Top %v 最后结果 %v---\n", conf.FinalCount*2, len(finalResults)))
	for i := 0; i < len(finalResults) && i < conf.FinalCount*2; i++ {
		WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency))
	}

	top := min(len(finalResults), conf.FinalCount*2)
	// 取前 outCount 名进行深度测速
	WriteLog(fmt.Sprintf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, conf.FinalCount))

	// 进行测速
	finalSorted := scanner.RunDeepTest(conf.FinalCount, sniDomain, conf.MinSpeed, finalResults, BridgeLogger)

	outPrefix := "result"
	jsonPath := "./okresult.json"
	// 假设结果已经存储在 finalSorted 切片中
	if len(finalSorted) > 0 {
		// 只有当搜到的 IP 数量大于 0 时，才覆盖旧的 result.json
		utils.SaveToCSV(outPrefix+".csv", finalSorted)
		utils.SaveToJSON(outPrefix+".json", finalSorted)

		err := utils.AppendToJSONFile(jsonPath, finalSorted)
		if err != nil {
			WriteLog(fmt.Sprintf("保存文件失败: %v\n", err))
		} else {
			WriteLog(fmt.Sprintf("结果已追加至: %s\n", jsonPath))
		}

		WriteLog(fmt.Sprintf("\n结果已保存至 %s.csv 和 %s.json\n", outPrefix, outPrefix))
	} else {
		WriteLog("本次未搜到优质 IP，保留旧的配置文件。")
	}

	WriteLog("\n✅ 优选后的 IP:")
	for i := 0; i < len(finalSorted); i++ {
		WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v  速度: %.2f MB/s\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs))
	}

	fmt.Println("\n✅ 最终优选建议:")
	if len(finalSorted) > 0 {
		WriteLog(fmt.Sprintf("最佳 IP: [%s] | 预估带宽: %.2f MB/s\n", finalSorted[0].IP, finalSorted[0].DownloadMBs))
	}

	// 扫描结束
	updateStatus(func(s *ScanStatus) {
		s.IsRunning = false
		//s.LastLog = "扫描完成"
	})
}
