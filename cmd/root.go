package cmd

import (
	"fmt"
	"os"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
	"github.com/spf13/cobra"
)

// 定义一个全局变量，初始为空。编译时 GitHub Actions 会把版本号注入到这里。
var Version = "dev-build"

// 定义所有参数对应的变量
var (
	fPath      string  // -f
	sniDomain  string  // -d
	minLatency int64   // -l
	nThreads   int     // -n
	outPrefix  string  // -o
	finalCount int     // -k
	jsonPath   string  // -p
	minSpeed   float64 // -s
	testNum    int     // -t
	appendMode bool    // -a
	showVer    bool    // -v
)

var rootCmd = &cobra.Command{
	Use:   "cf-scanner",
	Short: "高性能 Cloudflare IP 优选工具",
	Long: `🚀 Cloudflare IP Scanner (Cobra 版)
能够快速扫描 IP 段，根据延迟和下载速度筛选出最优质的 Cloudflare 节点。`,
	Run: func(cmd *cobra.Command, args []string) {
		// 优先处理版本号逻辑
		if showVer {
			fmt.Printf("cf-scanner version %s\n", Version)
			return
		}

		// 打印启动参数预览（可选）
		fmt.Printf("🎯 开始扫描任务...\n")
		fmt.Printf("   [文件]: %s  [并发]: %d  [目标]: %s\n", fPath, nThreads, sniDomain)
		fmt.Printf("   [过滤]: 延迟 <%dms, 保留的数量 %v \n\n", minLatency, finalCount)

		// --- 在这里调用你原来的核心扫描代码 ---
		ipGroups, actualTaskCount := utils.ParseIP(fPath, testNum)

		finalResults := scanner.RunScanPool(ipGroups, nThreads, sniDomain, minLatency, actualTaskCount)

		// 输出前 outCount 名
		fmt.Printf("\n--- 优选结果 Top %v 最后结果 %v---\n", finalCount*2, len(finalResults))
		for i := 0; i < len(finalResults) && i < finalCount*2; i++ {
			fmt.Printf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency)
		}

		top := finalCount * 2
		if len(finalResults) < finalCount*2 {
			top = len(finalResults)
		}
		// 取前 outCount 名进行深度测速
		fmt.Printf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, finalCount)

		finalSorted := scanner.RunDeepTest(finalCount, sniDomain, minSpeed, finalResults)

		// 假设结果已经存储在 finalSorted 切片中
		if len(finalSorted) > 0 {
			// 只有当搜到的 IP 数量大于 0 时，才覆盖旧的 result.json
			utils.SaveToCSV(outPrefix+".csv", finalSorted)
			utils.SaveToJSON(outPrefix+".json", finalSorted)
			if appendMode {
				err := utils.AppendToJSONFile(jsonPath, finalSorted)
				if err != nil {
					fmt.Printf("保存文件失败: %v\n", err)
				} else {
					fmt.Printf("结果已追加至: %s\n", jsonPath)
				}
			}
			fmt.Printf("\n结果已保存至 %s.csv 和 %s.json\n", outPrefix, outPrefix)
		} else {
			fmt.Println("本次未搜到优质 IP，保留旧的配置文件。")
		}

		fmt.Println("\n✅ 优选后的 IP:")
		for i := 0; i < len(finalSorted); i++ {
			fmt.Printf("排名 %d: [%s], 延迟: %v  速度: %.2f MB/s\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs)
		}

		fmt.Println("\n✅ 最终优选建议:")
		if len(finalSorted) > 0 {
			fmt.Printf("最佳 IP: [%s] | 预估带宽: %.2f MB/s\n", finalSorted[0].IP, finalSorted[0].DownloadMBs)
		}
	},
}

// Execute 供 main.go 调用
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// 1. 扫描与连接设置
	rootCmd.Flags().StringVarP(&fPath, "file", "f", "ip.txt", "包含 IP 段的文件路径")
	rootCmd.Flags().StringVarP(&sniDomain, "domain", "d", "speed.cloudflare.com/__down?bytes=100000000", "SNI 域名或测速下载链接")
	rootCmd.Flags().IntVarP(&nThreads, "threads", "n", 100, "并发协程数")
	rootCmd.Flags().IntVarP(&testNum, "test-num", "t", 500, "每个 IP 段抽样测试的 IP 数量")

	// 2. 过滤阈值设置
	rootCmd.Flags().Int64VarP(&minLatency, "latency", "l", 200, "最大允许延时 (ms)")
	rootCmd.Flags().Float64VarP(&minSpeed, "speed", "s", 5.0, "最低下载速度 (MB/s)")
	rootCmd.Flags().IntVarP(&finalCount, "out-num", "k", 100, "最终结果保留的数量")

	// 3. 输出与文件处理
	rootCmd.Flags().StringVarP(&outPrefix, "out-put", "o", "result", "输出 CSV、JSON 文件的路径前缀")
	rootCmd.Flags().StringVarP(&jsonPath, "push-json", "p", "./okresult.json", "输出到指定 JSON 文件的路径 (追加模式)")
	rootCmd.Flags().BoolVarP(&appendMode, "append", "a", false, "是否使用追加模式写入文件")

	// 4. 其他
	rootCmd.Flags().BoolVarP(&showVer, "version", "v", false, "显示版本号")

	// 如果你想修改默认的帮助信息展示，可以在这里微调
	rootCmd.Flags().SortFlags = false // 禁用按字母排序，改为按代码定义的顺序显示（更符合逻辑）
}
