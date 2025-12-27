package scanner

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// RunScanPool 启动并发扫描
func RunScanPool(ipGroups [][]string, workerCount int, domain string, latency int64, total int) []FinalResult {
	jobs := make(chan string, 200)
	resultsChan := make(chan FinalResult, 200)
	var wg sync.WaitGroup

	// 定义旋转字符
	var spinnerChars = []string{"\\", "|", "/", "-"}

	ctx, cancel := context.WithCancel(context.Background())
	go startSpinner(ctx, spinnerChars) // 启动旋转图标

	// 初始化进度条
	bar := progressbar.NewOptions(total,
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

	// 启动工人
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				// 调用同包下的 ScanIP
				res := ScanIP(ip, domain, 2*time.Second, latency)
				if res.isSuccess {
					resultsChan <- res
				}
				bar.Add(1)
			}
		}()
	}

	// 投放任务
	go func() {
		for _, group := range ipGroups {
			for _, ip := range group {
				jobs <- ip
			}
		}
		close(jobs)
	}()

	// 收集结果
	var finalResults []FinalResult
	done := make(chan struct{})
	go func() {
		for r := range resultsChan {
			finalResults = append(finalResults, r)
		}
		close(done)
	}()

	wg.Wait()
	cancel()        // 停止旋转图标
	fmt.Print("\r") // 结束后清除掉那个图标
	close(resultsChan)
	<-done // 等待结果切片填充完毕

	// 按延迟排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].RawLatency < finalResults[j].RawLatency
	})

	return finalResults
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

func RunDeepTest(outCount int, domain string, minSpeed float64, finalResults []FinalResult) []FinalResult {
	var finalSorted []FinalResult
	outResults := 0
	for i := 0; i < len(finalResults) && i < outCount*2; i++ {
		bestIP := finalResults[i].IP

		speed, err := TestSpeed(bestIP, domain, 5*time.Second)

		if err != nil {
			fmt.Printf("测速异常: %v\n", err)
			continue
		} else if speed < minSpeed {
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
		if outResults == outCount {
			i = outCount * 2
		}
	}

	// 按速度再次排序
	sort.Slice(finalSorted, func(i, j int) bool {
		return finalSorted[i].DownloadMBs > finalSorted[j].DownloadMBs
	})

	return finalSorted
}
