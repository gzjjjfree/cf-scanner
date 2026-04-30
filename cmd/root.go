package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/gzjjjfree/cf-scanner/utils"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// 定义一个全局变量，初始为空。编译时 GitHub Actions 会把版本号注入到这里。
var Version = "dev-build"

var rootCmd = &cobra.Command{
	Use:   "cf-scanner",
	Short: "高性能 Cloudflare IP 优选工具",
	Long: `🚀 Cloudflare IP Scanner (Cli 版)
能够快速扫描 IP 段，根据延迟和下载速度筛选出最优质的 Cloudflare 节点。
输出的结果文件可以当输入的 IP 列表文件进行精选
输出的 json 文件格式是 v2ray 改版 v5-result 读取 IP 池的格式`,
	Run: func(cmd *cobra.Command, args []string) {
		scanner.Conf.Check()

		// 是否显示 Web GUI
		//if scanner.Conf.ShowWeb && runtime.GOOS != "linux" {
		//	// 在另一个线程启动 Web 服务，防止阻塞
		//	go func() {
		//		webUI.Makeweb()
		//	}()
		//
		//	// 给服务器一点启动时间（比如 500ms），然后打开浏览器
		//	time.Sleep(time.Millisecond * 500)
		//	webUI.OpenBrowser("http://127.0.0.1:8090")
		//	// 保持主进程不退出
		//	select {}
		//}

		// 优先处理版本号逻辑
		if scanner.Conf.ShowVer {
			fmt.Printf("cf-scanner version %s\n", Version)
			return
		}

		scanner.StatusMutex.Lock()
		var ctx context.Context
		ctx, scanner.CancelScan = context.WithCancel(context.Background())
		scanner.StatusMutex.Unlock()

		keyDone := make(chan struct{})
		// 启动一个后台协程专门盯着键盘
		go scanner.ListenForStopKey(ctx, scanner.CancelScan, keyDone)

		// 优先处理版本号逻辑
		if scanner.Conf.DownloadV5 {
			fmt.Printf("正在下载 v5-result\n")
			targetURL := utils.GetDownloadURL("https://github.com/gzjjjfree/v5-result/releases/download", "custom-build", "v5-result") // 这里可以换成你动态获取的最新版本号
			fileName := path.Base(targetURL)

			// 1. 获取可执行文件所在的目录
			exePath, _ := os.Executable()
			exeDir := filepath.Dir(exePath) // 获取 /home/.../build/bin 这个目录

			// 2. 拼接完整的目标路径
			fullPath := filepath.Join(exeDir, fileName)
			fmt.Println("下载目录在: ", exeDir)
			err := utils.DownloadFile(ctx, targetURL, fullPath, BridgeLogger)
			if err == nil {
				fmt.Println(utils.DownloadMsg)
			}
			scanner.CancelScan()
			<-keyDone // 阻塞等待，直到 keyboard.Close() 执行完毕
			return
		}

		if !scanner.Conf.ShouldRun {
			// 如果用户没传 --run，则打印帮助并退出
			cmd.Help()
			return
		}

		defer func() {
			scanner.StatusMutex.Lock()
			defer scanner.StatusMutex.Unlock()
			scanner.Status.WaitStop = false
		}()

		// 打印启动参数预览（可选）
		fmt.Printf("🎯 开始扫描任务...\n")
		fmt.Printf("   [文件]: %s  [并发]: %d  [抽样]: %d/段  [目标]: %s\n", scanner.Conf.FilePath, scanner.Conf.NThreads, scanner.Conf.TestNum, scanner.Conf.SniDomain)
		fmt.Printf("   [过滤]: 延迟 <%dms, 最低下载速度 >%v, 保留的数量 %v \n\n", scanner.Conf.MinLatency, scanner.Conf.MinSpeed, scanner.Conf.FinalCount)

		scanner.StatusMutex.Lock()
		scanner.Status.WaitStop = false
		scanner.StatusMutex.Unlock()

		// 对 IP 段进行随机抽样
		ipGroups, actualTaskCount := utils.ParseIP(ctx, scanner.Conf.FilePath, scanner.Conf.TestNum, BridgeLogger)
		if actualTaskCount <= 0 {
			fmt.Println("读取 IP 文件出错，结束扫描！")
			return
		}

		// 扫描延迟
		finalResults := scanner.RunScanPool(ctx, ipGroups, scanner.Conf.NThreads, scanner.Conf.SniDomain, scanner.Conf.MinLatency, actualTaskCount, BridgeLogger)

		// 输出前 outCount 名
		//fmt.Printf("\n--- 优选结果 Top %v 最后结果 %v---\n", scanner.Conf.FinalCount*2, len(finalResults))
		//for i := 0; i < len(finalResults) && i < scanner.Conf.FinalCount*2; i++ {
		//	fmt.Printf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency)
		//}

		scanner.StatusMutex.Lock()
		if scanner.Status.WaitStop {
			defer scanner.StatusMutex.Unlock()
			scanner.Status.WaitStop = false
			return
		}
		scanner.StatusMutex.Unlock()

		//top := min(len(finalResults), max(scanner.Conf.FinalCount*2, 100))
		// 取前 outCount 名进行深度测速
		//fmt.Printf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, scanner.Conf.FinalCount)
		fmt.Printf("\n--- 开始进行下载测速，优选 %v 个结果 ---\n", scanner.Conf.FinalCount)
		
		// 进行测速
		finalSorted := scanner.RunDeepTest(ctx, scanner.Conf.FinalCount, scanner.Conf.SniDomain, scanner.Conf.MinSpeed, finalResults, BridgeLogger)

		// 假设结果已经存储在 finalSorted 切片中
		if len(finalSorted) > 0 {
			// 只有当搜到的 IP 数量大于 0 时，才覆盖旧的 result.json
			utils.SaveToCSV(scanner.Conf.OutPrefix+".csv", finalSorted)
			utils.SaveToJSON(scanner.Conf.OutPrefix+".json", finalSorted)
			if scanner.Conf.AppendMode {
				err := utils.AppendToJSONFile(scanner.Conf.JsonPath, finalSorted)
				if err != nil {
					fmt.Printf("保存文件失败: %v\n", err)
				} else {
					fmt.Printf("结果已追加至: %s\n", scanner.Conf.JsonPath)
				}
			}
			fmt.Printf("\n结果已保存至 %s.csv 和 %s.json\n", scanner.Conf.OutPrefix, scanner.Conf.OutPrefix)
		} else {
			fmt.Println("\n本次未搜到优质 IP，保留旧的配置文件。")
		}

		fmt.Println("\n✅ 优选后的 IP:")
		for i := 0; i < len(finalSorted); i++ {
			fmt.Printf("排名 %d: [%s], 延迟: %v  速度: %.2f MB/s\n", i+1, finalSorted[i].IP, finalSorted[i].Latency, finalSorted[i].DownloadMBs)
		}

		fmt.Println("\n✅ 最终优选建议:")
		if len(finalSorted) > 0 {
			fmt.Printf("最佳 IP: [%s] | 预估带宽: %.2f MB/s\n", finalSorted[0].IP, finalSorted[0].DownloadMBs)
		}
		scanner.CancelScan()
		<-keyDone // 阻塞等待，直到 keyboard.Close() 执行完毕
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
	rootCmd.Flags().StringVarP(&scanner.Conf.FilePath, "file", "f", "ip.txt", "包含 IP 段的文件路径")
	rootCmd.Flags().StringVarP(&scanner.Conf.SniDomain, "domain", "d", "speed.cloudflare.com/__down?bytes=100000000", "SNI 域名或测速下载链接")
	rootCmd.Flags().IntVarP(&scanner.Conf.NThreads, "threads", "n", 100, "并发协程数")
	rootCmd.Flags().IntVarP(&scanner.Conf.TestNum, "test-num", "t", 500, "每个 IP 段抽样测试的 IP 数量")

	// 2. 过滤阈值设置
	rootCmd.Flags().Int64VarP(&scanner.Conf.MinLatency, "latency", "l", 200, "最大允许延时 (ms)")
	rootCmd.Flags().Float64VarP(&scanner.Conf.MinSpeed, "speed", "s", 5.0, "最低下载速度 (MB/s)")
	rootCmd.Flags().IntVarP(&scanner.Conf.FinalCount, "out-num", "k", 10, "最终结果保留的数量")

	// 3. 输出与文件处理
	rootCmd.Flags().StringVarP(&scanner.Conf.OutPrefix, "out-put", "o", "result/result", "输出 CSV、JSON 文件的路径前缀")
	rootCmd.Flags().StringVarP(&scanner.Conf.JsonPath, "push-json", "p", "./okresult.json", "输出到指定 JSON 文件的路径 (追加模式)")
	rootCmd.Flags().BoolVarP(&scanner.Conf.AppendMode, "append", "a", false, "是否使用追加模式写入文件")
	rootCmd.Flags().BoolVarP(&scanner.Conf.DownloadV5, "downloadv5", "y", false, "是否下载 v2ray 定制版 v5-result")

	// 4. 其他
	rootCmd.Flags().BoolVarP(&scanner.Conf.ShowVer, "version", "v", false, "显示版本号")
	//rootCmd.Flags().BoolVarP(&scanner.Conf.ShowWeb, "web", "w", false, "显示 Web GUI")
	rootCmd.Flags().BoolVarP(&scanner.Conf.ShouldRun, "run", "r", false, "正式开始运行扫描任务")

	// 如果你想修改默认的帮助信息展示，可以在这里微调
	rootCmd.Flags().SortFlags = false // 禁用按字母排序，改为按代码定义的顺序显示（更符合逻辑）
}

// 定义一个空的结构体，作为接口的载体
type rootLogger struct {
	Theme progressbar.Theme
}

// 让 testLogger 实现 WriteLog 方法
func (w rootLogger) WriteLog(msg string) {
	fmt.Print(msg)
}

func (w rootLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w rootLogger) GetColorCodes() bool {
	return true
}

var BridgeLogger = rootLogger{
	Theme: progressbar.Theme{
		Saucer:        "[green]=[reset]",
		SaucerHead:    "[cyan]>[reset]",
		SaucerPadding: " ",
		BarStart:      "[blue][[reset]",
		BarEnd:        "[blue]][reset]",
	},
}
