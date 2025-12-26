package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// 定义一个全局变量，初始为空。编译时 GitHub Actions 会把版本号注入到这里。
var version = "v0.0.0"

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 1. 定义命令行参数
	domain := flag.String("d", "speed.cloudflare.com/__down?bytes=100000000", "测试的域名 (SNI)")
	ipFile := flag.String("f", "ip.txt", "包含 IP 段的文件路径")
	outFile := flag.String("o", "result", "输出文件路径加前缀 (不带后缀)")
	workerCount := flag.Int("n", 100, "并发协程数")
	latency := flag.Int64("l", 200, "最低延时")
	minSpeed := flag.Float64("s", 10, "最低下载")
	outCount := flag.Int("on", 100, "最终结果数")
	testCount := flag.Int("tn", 500, "单个 IP 段期望测试的 IP 数量")
	help := flag.Bool("h", false, "显示帮助信息")
	showVersion := flag.Bool("v", false, "显示版本号")
	outputFile := flag.String("p", "./okresult.json", "输出到指定 JSON 文件（追加模式）")
	appendMode := flag.Bool("a", false, "是否使用追加模式写入文件")

	// 2. 自定义帮助信息显示方式
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Cloudflare 优选 IP 扫描工具\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n  ./cf-scanner [options]\n\n")
		fmt.Fprintf(os.Stderr, "参数说明:\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  -%-10s %s (默认值: %v)\n", f.Name, f.Usage, f.DefValue)
		})
		fmt.Fprintf(os.Stderr, "\n示例:\n  ./cf-scanner -d www.speed.com/10mb.bin -o c:\\ips\n")
	}

	flag.Parse()

	// 如果用户输入了 -help
	if *help {
		flag.Usage()
		return
	}

	if *showVersion {
		fmt.Printf("cf-scanner 版本: %s\n", version)
		os.Exit(0)
	}

	// 3. 读取并解析 IP 段文件
	cidrList, isJSONInput, err := readLines(*ipFile)
	if err != nil {
		fmt.Printf("无法读取 IP 文件: %v\n", err)
		return
	}

	// 4. 每段分别取样
	ipGroups := make([][]string, 1)
	for _, cidr := range cidrList {
		ips, _ := ParseCIDR(cidr)
		if isJSONInput {
			// json 文件全部 ip 读入groups[0]
			ipGroups[0] = append(ipGroups[0], ips...)
		} else {
			// 每个 ip 段分别取样
			groups := pickSamples(ips, *testCount)
			fmt.Printf("IP 段 [%v] 随机抽样数为: %v\n", cidr, len(groups))
			// 二维切片 ipGroups 的每个切片都是一个 ip 段取样的结果
			ipGroups = append(ipGroups, groups)
		}
	}

	// 5. 预计算总数 (非常重要！)
	actualTaskCount := 0
	for i := 0; i < len(ipGroups); i++ {
		for o := 0; o < len(ipGroups[i]); o++ {
			actualTaskCount++
		}
	}

	fmt.Printf("解析完成，总计 %d 个 IP，开始随机抽样扫描...\n", actualTaskCount)

	// 6. 定义旋转字符
	var spinnerChars = []string{"\\", "|", "/", "-"}

	ctx, cancel := context.WithCancel(context.Background())
	go startSpinner(ctx, spinnerChars) // 启动旋转图标

	// 7. 初始化进度条
	bar := progressbar.NewOptions(actualTaskCount,
		progressbar.OptionSetDescription("    正在扫描 IP"),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false), // 扫描不是字节，关闭它
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// 8. 建立任务通道
	jobs := make(chan string, 200)
	results := make(chan FinalResult, 200)
	var wg sync.WaitGroup

	// 9. 启动工人 (Goroutines)
	for i := 0; i < *workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				scanTimeout := 2 * time.Second
				res := ScanIP(ip, *domain, scanTimeout, *latency)
				if res.isSuccess { // 只有成功的 IP 才进入结果集
					results <- res
				}
				// 每次接收到一个结果，进度条前进一格
				bar.Add(1)
			}
		}()
	}

	for _, group := range ipGroups {
		for _, ip := range group {
			jobs <- ip
		}
	}

	close(jobs)

	// 10. 等待工人干完活并收集结果
	go func() {
		wg.Wait()
		fmt.Println("\n✅ 扫描完成！")
		close(results)
	}()

	wg.Wait()       // 等待工人完成
	cancel()        // 停止旋转图标
	fmt.Print("\r") // 结束后清除掉那个图标

	var finalResults []FinalResult
	for r := range results {
		finalResults = append(finalResults, r)
	}

	// 11. 按延迟排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].RawLatency < finalResults[j].RawLatency
	})

	// 12. 输出前 outCount 名
	fmt.Printf("\n--- 优选结果 Top %v 最后结果 %v---\n", *outCount*2, len(finalResults))
	for i := 0; i < len(finalResults) && i < *outCount*2; i++ {
		fmt.Printf("排名 %d: [%s], 延迟: %v\n", i+1, finalResults[i].IP, finalResults[i].Latency)
	}

	top := *outCount * 2
	if len(finalResults) < *outCount*2 {
		top = len(finalResults)
	}
	// 13. 取前 outCount 名进行深度测速
	fmt.Printf("\n--- 开始对 Top %v 进行下载测速，优选 %v 个结果 ---\n", top, *outCount)
	var finalSorted []FinalResult
	outResults := 0
	for i := 0; i < len(finalResults) && i < *outCount*2; i++ {
		bestIP := finalResults[i].IP

		speed, err := TestSpeed(bestIP, *domain, 5*time.Second)

		if err != nil {
			fmt.Printf("测速异常: %v\n", err)
			continue
		} else if speed < *minSpeed {
			fmt.Printf("速率过低: [%s] 速度: %.2f Mbps\n", bestIP, speed)
			continue
		} else {
			fmt.Printf("🚀 [%s] 速度: %.2f Mbps\n", bestIP, speed)
		}

		finalSorted = append(finalSorted, FinalResult{
			IP:          bestIP,
			DownloadMBs: speed,                   // 对应结构体中的 DownloadMBs 字段
			Latency:     finalResults[i].Latency, // 别忘了把第一轮测得的延迟也带过来，方便存入 CSV
			CreatedAt:   time.Now(),              // 记录这一刻的时间
		})

		outResults++
		if outResults == *outCount {
			i = *outCount * 2
		}
	}

	// 14. 按速度再次排序
	sort.Slice(finalSorted, func(i, j int) bool {
		return finalSorted[i].DownloadMBs > finalSorted[j].DownloadMBs
	})

	// 15. 假设结果已经存储在 finalSorted 切片中
	if len(finalSorted) > 0 {
		// 只有当搜到的 IP 数量大于 0 时，才覆盖旧的 result.json
		saveToCSV(*outFile+".csv", finalSorted)
		saveToJSON(*outFile+".json", finalSorted)
		if *appendMode {
			err := appendToJSONFile(*outputFile, finalSorted)
			if err != nil {
				fmt.Printf("保存文件失败: %v\n", err)
			} else {
				fmt.Printf("结果已追加至: %s\n", *outputFile)
			}
		}
		fmt.Printf("\n结果已保存至 %s.csv 和 %s.json\n", *outFile, *outFile)
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

// saveToCSV 保存详细报告
func saveToCSV(filename string, data []FinalResult) {
	file, _ := os.Create(filename)
	defer file.Close()
	file.WriteString("\xEF\xBB\xBF") // 写入 UTF-8 BOM

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"IP 地址", "延迟", "下载速度", "时间"})
	for _, r := range data {
		writer.Write([]string{
			r.IP,
			r.Latency,
			fmt.Sprintf("%.2f", r.DownloadMBs),
			r.CreatedAt.Format("2006-01-02 15:04:05"), // Go 的标准时间格式化写法
		})
	}
}

// saveToJSON 仅保存地址列表
func saveToJSON(filename string, data []FinalResult) {
	file, _ := os.Create(filename)
	defer file.Close()

	// 如果你只需要 JSON 里显示 address 字段，
	// FinalResult 里的其他字段在定义时加了 omitempty，且没有赋值时就会被隐藏
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	encoder.Encode(data)
}

// ip 段取样
func pickSamples(ips []string, testCount int) []string {
	// 引入随机步长
	targetCount := testCount // 我们希望最终测试的 IP 数量
	var currentStep int

	totalIPs := len(ips)
	if totalIPs <= targetCount {
		// 如果 IP 总数还没到希望最终测试的数量，没必要抽样，直接全测
		currentStep = 1
	} else {
		// 自动计算步长：总数 / 目标数
		// 例如：500,000 / 200 = 2500 (步长)
		currentStep = totalIPs / targetCount
	}

	var sampled []string

	for i := 0; i < totalIPs; i += currentStep {
		// 计算当前区间的结束位置
		end := i + currentStep
		if end > totalIPs {
			end = totalIPs
		}

		// 在 [i, end) 区间内随机选一个索引
		randomIndex := i + rand.Intn(end-i)
		sampled = append(sampled, ips[randomIndex])
	}

	return sampled
}

func startSpinner(ctx context.Context, spinnerChars []string) {
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 使用 \r 回到行首，打印图标
			// 注意：如果后面有进度条，需确保不会覆盖掉进度条的内容
			fmt.Printf("\r%s ", spinnerChars[i%len(spinnerChars)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func appendToJSONFile(path string, newResults []FinalResult) error {
	var existingData []map[string]interface{}

	// 1. 尝试读取现有文件
	fileData, err := os.ReadFile(path)
	if err == nil && len(fileData) > 0 {
		// 如果文件存在且不为空，解析现有内容
		if err := json.Unmarshal(fileData, &existingData); err != nil {
			// 如果解析失败，说明原文件可能不是合法的 JSON 数组，记录警告
			fmt.Printf("警告: 原文件格式不兼容，将创建新数组: %v\n", err)
			existingData = []map[string]interface{}{}
		}
	}

	// 2. 将新结果转换为 map 结构（为了只保留带 json 标签的字段）
	// 这样做可以确保忽略那些标记为 `json:"-"` 的字段
	for _, res := range newResults {
		// 我们通过这种方式只提取带 json 标签的字段
		item := map[string]interface{}{
			"address": res.IP,
		}

		// 可选：在这里做去重逻辑
		isDuplicate := false
		for _, existing := range existingData {
			if existing["address"] == res.IP {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			existingData = append(existingData, item)
		}
	}

	// 3. 序列化回 JSON 数组（带缩进方便阅读）
	updatedJSON, err := json.MarshalIndent(existingData, "", "    ")
	if err != nil {
		return err
	}

	// 4. 覆盖写入文件
	return os.WriteFile(path, updatedJSON, 0644)
}
