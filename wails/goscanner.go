package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
	"github.com/schollz/progressbar/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 方便外部调用的实例, 传参给外部让外部可以使用本包的方法
var BridgeLogger = WailsLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

// 定义发送给前端的消息结构
type WSMessage struct {
	Type string `json:"type"` // "log" 或 "status"
	Data any    `json:"data"` // 日志内容 或 状态对象
}

// 启动流程
func startScanWorkflow(a *SimpleApp, params map[string]string) {
	scanner.StatusMutex.Lock()
	if scanner.Status.IsRunning {
		scanner.StatusMutex.Unlock()
		return
	}
	var appCtx context.Context
	scanner.Status.IsRunning = true
	scanner.Status.WaitStop = false
	appCtx, scanner.CancelScan = context.WithCancel(a.ctx)
	scanner.StatusMutex.Unlock()

	// 转换 Threads (string -> int)
	if val, ok := params["threads"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 1 && i < 101) {
			scanner.Conf.NThreads = i
		}
	}

	// 转换 MinSpeed (string -> float64)
	if val, ok := params["min_speed"]; ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil && (f > 0.1 && f < 21) {
			scanner.Conf.MinSpeed = f
		}
	}

	// 转换 MinLatency (string -> int)
	if val, ok := params["min_latency"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 10 && i < 1001) {
			scanner.Conf.MinLatency = int64(i)
		}
	}

	// 转换 FinalCount (string -> int)
	if val, ok := params["final_count"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 1 && i < 501) {
			scanner.Conf.FinalCount = i
		}
	}

	// 转换 TestNum (string -> int)
	if val, ok := params["test_num"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 10 && i < 2001) {
			scanner.Conf.TestNum = i
		}
	}

	scanner.Conf.Check()

	runScannerLogic(appCtx, scanner.Conf)
}

func runScannerLogic(appCtx context.Context, conf scanner.ScanConfig) {
	// 告诉前端：扫描开始！
	broadcastStatus(appCtx, scanner.Status.IsRunning)
	// 无论正常结束还是取消，最后都告诉前端：停下来。
	defer func() {
		scanner.StatusMutex.Lock()
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		broadcastStatus(appCtx, scanner.Status.IsRunning)
	}()

	// 扫描过程
	BridgeLogger.Ctx = appCtx

	BridgeLogger.WriteLog("🚀 扫描任务启动...\n")
	BridgeLogger.WriteLog(fmt.Sprintf("[文件]: %s  [并发]: %d  [抽样]: %d/段  [目标]: %s\n", conf.FilePath, conf.NThreads, conf.TestNum, conf.SniDomain))
	BridgeLogger.WriteLog(fmt.Sprintf("[过滤]: 延迟 <%dms, 最低下载速度 >%v, 保留的数量 %v \n\n", conf.MinLatency, conf.MinSpeed, conf.FinalCount))

	ipGroups, actualTaskCount := utils.ParseIP(conf.FilePath, conf.TestNum, BridgeLogger)
	if actualTaskCount <= 0 {
		BridgeLogger.WriteLog(fmt.Sprintln("读取 IP 文件出错，结束扫描！"))
		return
	}

	finalResults := scanner.RunScanPool(appCtx, ipGroups, conf.NThreads, conf.SniDomain, int64(conf.MinLatency), actualTaskCount, BridgeLogger)

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
	finalSorted := scanner.RunDeepTest(appCtx, conf.FinalCount, conf.SniDomain, conf.MinSpeed, finalResults, BridgeLogger)

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

	BridgeLogger.WriteLog("\n✅ 优选后的 IP:\nn")
	for i := 0; i < len(finalSorted); i++ {
		BridgeLogger.WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v  速度: %.2f MB/s\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs))
	}

	BridgeLogger.WriteLog("\n✅ 最终优选建议:\n")
	if len(finalSorted) > 0 {
		BridgeLogger.WriteLog(fmt.Sprintf("最佳 IP: [%s] | 预估带宽: %.2f MB/s\n", finalSorted[0].IP, finalSorted[0].DownloadMBs))
	}
	// 扫描结束
}

// 发送状态更新（在扫描开始和结束时调用）
func broadcastStatus(ctx context.Context, isScanning bool) {
	msg := WSMessage{
		Type: "status",
		Data: map[string]bool{"is_scanning": isScanning},
	}

	runtime.EventsEmit(ctx, "scan_log", msg)
	scanner.Status.IsRunning = isScanning
}
