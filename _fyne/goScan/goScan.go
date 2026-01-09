package goScan

import (
	"context"
	"fmt"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
)

// 整理参数
func ReceivingParameters(threads int, testNum int, latency int64, speed float64) {
	conf.NThreads = threads
	conf.TestNum = testNum
	conf.MinLatency = latency
	conf.MinSpeed = speed

	conf.Check()

	scanner.StatusMutex.Lock()
	if scanner.Status.IsRunning {
		scanner.StatusMutex.Unlock()
		return
	}

	scanner.Status.IsRunning = true
	scanner.Status.WaitStop = false
	scanner.StatusMutex.Unlock()

	runScannerLogic()
}

func runScannerLogic() {
	defer func() {
		scanner.StatusMutex.Lock()
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		endScanner("\n✨ 扫描任务已结束。\n")
	}()
	
	scanner.StatusMutex.Lock()
	var ctx context.Context
	ctx, scanner.CancelScan = context.WithCancel(context.Background())
	BridgeLogger.Ctx = ctx
	scanner.StatusMutex.Unlock()

	BridgeLogger.WriteLog("🚀 扫描任务启动...\n")
	BridgeLogger.WriteLog(fmt.Sprintf("[文件]: %s  [并发]: %d  [抽样]: %d/段  [目标]: %s\n", conf.FilePath, conf.NThreads, conf.TestNum, conf.SniDomain))
	BridgeLogger.WriteLog(fmt.Sprintf("[过滤]: 延迟 <%dms, 最低下载速度 >%v, 保留的数量 %v \n\n", conf.MinLatency, conf.MinSpeed, conf.FinalCount))

	ipGroups, actualTaskCount := utils.ParseIP(conf.FilePath, conf.TestNum, BridgeLogger)
	if actualTaskCount <= 0 {
		BridgeLogger.WriteLog(fmt.Sprintln("读取 IP 文件出错，结束扫描！"))
		return
	}

	finalResults := scanner.RunScanPool(ctx, ipGroups, conf.NThreads, conf.SniDomain, int64(conf.MinLatency), actualTaskCount, BridgeLogger)

	if ctx.Err() != nil {
		endScanner("\n🛑 扫描已提前停止！不进行下一步测速\n")
		return
	}

	BridgeLogger.WriteLog(fmt.Sprintf("\n--- 优选结果 Top %v 最后结果 %v---\n", conf.FinalCount*2, len(finalResults)))

	for i := 0; i < len(finalResults) && i < conf.FinalCount*2; i++ {
		BridgeLogger.WriteLog(fmt.Sprintf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency))
	}

	scanner.StatusMutex.Lock()
	if scanner.Status.WaitStop {
		defer scanner.StatusMutex.Unlock()
		scanner.Status.IsRunning = false
		scanner.Status.WaitStop = false
		endScanner("\n🛑 扫描已提前停止！不进行下一步测速\n")
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
		utils.SaveToCSV(conf.OutPrefix + ".csv", finalSorted)
		utils.SaveToJSON(conf.OutPrefix + ".json", finalSorted)

		err := utils.AppendToJSONFile(conf.JsonPath, finalSorted)
		if err != nil {
			BridgeLogger.WriteLog(fmt.Sprintf("\n保存文件失败: %v\n", err))
		} else {
			BridgeLogger.WriteLog(fmt.Sprintf("\n结果已追加至: %s\n", conf.JsonPath))
		}

		BridgeLogger.WriteLog(fmt.Sprintf("结果已保存至 %s.csv 和 %s.json\n", conf.OutPrefix, conf.OutPrefix))
	} else {
		endScanner("\n本次未搜到优质 IP，保留旧的配置文件。")
		return
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

func endScanner(msg string) {
	BridgeLogger.WriteLog(msg)
	FinishChan <- scanner.Status.IsRunning
}
