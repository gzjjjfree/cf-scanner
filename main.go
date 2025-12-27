package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
)

// 定义一个全局变量，初始为空。编译时 GitHub Actions 会把版本号注入到这里。
var version = "v0.0.0"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 参数解析 (保持在最前面，防止杀毒软件扫描延迟)
	conf := utils.ParseConfig()

	if conf.ShowVersion {
		fmt.Printf("cf-scanner 版本: %s\n", version)
		return
	}

	// 如果用户输入了 -help
	if conf.Help {
		flag.Usage()
		return
	}

	// 自定义帮助信息显示方式
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Cloudflare 优选 IP 扫描工具\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n  ./cf-scanner [options]\n\n")
		fmt.Fprintf(os.Stderr, "参数说明:\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  -%-10s %s (默认值: %v)\n", f.Name, f.Usage, f.DefValue)
		})
		fmt.Fprintf(os.Stderr, "\n示例:\n  ./cf-scanner -d www.speed.com/10mb.bin -o c:\\ips\n")
	}

	ipGroups, actualTaskCount := utils.ParseIP(conf)

	finalResults := scanner.RunScanPool(ipGroups, conf.WorkerCount, conf.Domain, conf.LatencyLimit, actualTaskCount)

	// 输出前 outCount 名
	fmt.Printf("\n--- 优选结果 Top %v 最后结果 %v---\n", conf.OutCount*2, len(finalResults))
	for i := 0; i < len(finalResults) && i < conf.OutCount*2; i++ {
		fmt.Printf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency)
	}

	top := conf.OutCount * 2
	if len(finalResults) < conf.OutCount*2 {
		top = len(finalResults)
	}
	// 取前 outCount 名进行深度测速
	fmt.Printf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, conf.OutCount)

	finalSorted := scanner.RunDeepTest(conf.OutCount, conf.Domain, conf.MinSpeed, finalResults)

	// 假设结果已经存储在 finalSorted 切片中
	if len(finalSorted) > 0 {
		// 只有当搜到的 IP 数量大于 0 时，才覆盖旧的 result.json
		utils.SaveToCSV(conf.OutFile+".csv", finalSorted)
		utils.SaveToJSON(conf.OutFile+".json", finalSorted)
		if conf.AppendMode {
			err := utils.AppendToJSONFile(conf.OutputFilePath, finalSorted)
			if err != nil {
				fmt.Printf("保存文件失败: %v\n", err)
			} else {
				fmt.Printf("结果已追加至: %s\n", conf.OutputFilePath)
			}
		}
		fmt.Printf("\n结果已保存至 %s.csv 和 %s.json\n", conf.OutFile, conf.OutFile)
	} else {
		fmt.Println("本次未搜到优质 IP，保留旧的配置文件。")
	}

	fmt.Println("\n✅ 优选后的 IP:")
	for i := 0; i < len(finalSorted); i++ {
		fmt.Printf("排名 %d: [%s], 延迟: %v  速度: %.2f Mbps\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs)
	}

	fmt.Println("\n✅ 最终优选建议:")
	if len(finalSorted) > 0 {
		fmt.Printf("最佳 IP: [%s] | 预估带宽: %.2f Mbps\n", finalSorted[0].IP, finalSorted[0].DownloadMBs)
	}
}
