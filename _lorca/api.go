package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
)

func registerFuns() {
	// 绑定开始扫描的函数
	ui.Bind("apiStartScan", func(cmd scanner.WSMessage) string {
		fmt.Println("接收到的信息是：", cmd.Type)
		// 根据 Type 分发逻辑
		switch cmd.Type {
		case "start":
			// 1. 首先断言 cmd.Data 是一个 map
			rawParams, ok := cmd.Data.(map[string]any)
			if !ok {
				BridgeLogger.WriteLog("❌ 错误：无效的参数格式\n")
				scanner.Status.IsRunning = false
				handleScanUpdate(false)
				break
			}

			// 2. 创建一个真正的 map[string]string
			cleanParams := make(map[string]string)
			for k, v := range rawParams {
				// 将 any 转换为 string（处理字符串、数字、布尔等）
				cleanParams[k] = fmt.Sprintf("%v", v)
			}
			// 你可以从 cmd.Type 中提取前端传来的参数
			go func() {
				startScanWorkflow(cleanParams)
			}()
		case "stop":
			if scanner.CancelScan != nil {
				scanner.CancelScan()
				scanner.Status.WaitStop = true
				BridgeLogger.WriteLog("\n🛑 正在通过 WebSocket 指令停止任务...\n")
			}
		}

		return "OK" // 立即返回给前端，表示后台已开始运行
	})
	ui.Bind("apiStopScan", func(cmd scanner.WSMessage) string {
		return "OK"
	})
}

// 启动前处理参数
func startScanWorkflow(params map[string]string) {
	scanner.StatusMutex.Lock()
	if scanner.Status.IsRunning {
		scanner.StatusMutex.Unlock()
		return
	}
	var scanCtx context.Context
	scanCtx, scanner.CancelScan = context.WithCancel(context.Background())
	scanner.Status.IsRunning = true
	scanner.Status.WaitStop = false
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

	runScannerLogic(scanCtx, scanner.Conf)
}

func runScannerLogic(ctx context.Context, conf scanner.ScanConfig) {
	// 告诉前端：扫描开始！
	handleScanUpdate(scanner.Status.IsRunning)
	// 无论正常结束还是取消，最后都告诉前端：停下来。
	defer func() {
		scanner.StatusMutex.Lock()
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		handleScanUpdate(scanner.Status.IsRunning)
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

	BridgeLogger.WriteLog("\n✅ 优选后的 IP:\n")
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
func handleScanUpdate(isScanning bool) {
	msg := scanner.WSMessage{
		Type: "status",
		Data: map[string]bool{"is_scanning": isScanning},
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("JSON 序列化失败: %v\n", err)
		return
	}

	ui.Eval(fmt.Sprintf(`window.handleScanUpdate(%s)`, string(jsonData)))
}
