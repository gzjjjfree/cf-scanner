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

	StatusMutex.Lock()
	if Status.isRunning {
		StatusMutex.Unlock()
		return
	}

	Status.isRunning = true
	Status.WaitStop = false
	StatusMutex.Unlock()

	defer func() {
		StatusMutex.Lock()
		Status.isRunning = false
		Status.WaitStop = false
		StatusMutex.Unlock()
	}()

	go runScannerLogic()
}

func runScannerLogic() {
	ctx, cancel := context.WithCancel(context.Background())
	BridgeLogger.Ctx = ctx
	scanner.CancelScan = cancel

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

	if Status.WaitStop {
		endScanner("\n🛑 扫描已提前停止！不进行下一步测速\n")
		return
	}

	top := min(len(finalResults), conf.FinalCount*2)
	// 取前 outCount 名进行深度测速
	BridgeLogger.WriteLog(fmt.Sprintf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, conf.FinalCount))

	// 进行测速
	finalSorted := scanner.RunDeepTest(ctx, conf.FinalCount, conf.SniDomain, conf.MinSpeed, finalResults, BridgeLogger)

	outPrefix := "result"
	jsonPath := "./okresult.json"
	// 结果已经存储在 finalSorted 切片中
	if len(finalSorted) > 0 {
		utils.SaveToCSV(outPrefix+".csv", finalSorted)
		utils.SaveToJSON(outPrefix+".json", finalSorted)

		err := utils.AppendToJSONFile(jsonPath, finalSorted)
		if err != nil {
			BridgeLogger.WriteLog(fmt.Sprintf("\n保存文件失败: %v\n", err))
		} else {
			BridgeLogger.WriteLog(fmt.Sprintf("\n结果已追加至: %s\n", jsonPath))
		}

		BridgeLogger.WriteLog(fmt.Sprintf("结果已保存至 %s.csv 和 %s.json\n", outPrefix, outPrefix))
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

	endScanner("")
}

func endScanner(msg string) {
	BridgeLogger.WriteLog(msg)
	Status.WaitStop = false
	Status.isRunning = false
	FinishChan <- Status.isRunning
}
