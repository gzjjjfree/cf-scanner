package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/schollz/progressbar/v3"
)

type writerWrapper struct {
	l Logger
}

func (w writerWrapper) Write(p []byte) (n int, err error) {
	// 将进度条发送过来的 []byte 转换为 string，并调用你的接口方法
	msg := string(p)

	w.l.WriteLog(msg)

	return len(p), nil
}

// RunScanPool 启动并发扫描
func RunScanPool(ctx context.Context, ipGroups [][]string, workerCount int, domain string, latency int64, total int, l Logger) []FinalResult {
	jobs := make(chan string, 200)
	resultsChan := make(chan FinalResult, 200)
	var wg sync.WaitGroup

	var bar *progressbar.ProgressBar

	// 定义一个适配器，把 logger 包装成 io.Writer
	loggerAdapter := writerWrapper{l: l}

	// 初始化进度条
	bar = progressbar.NewOptions(total,
		progressbar.OptionSetWriter(loggerAdapter),
		progressbar.OptionSetDescription("    正在扫描 IP"),
		progressbar.OptionEnableColorCodes(l.GetColorCodes()),
		progressbar.OptionShowBytes(false), // 扫描不是字节，关闭它
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetTheme(l.GetTheme()),
	)

	defer bar.Close()
	// 启动工人
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done(): // 工人在处理每个 IP 前都会检查是否已停止
					return
				case ip, ok := <-jobs:
					if !ok {
						return
					}
					// 执行扫描
					res := ScanIP(ctx, ip, domain, 2*time.Second, latency)

					// 再次检查，防止在 ScanIP 完成期间 ctx 被取消
					if ctx.Err() != nil {
						return
					}

					if res.isSuccess {
						resultsChan <- res
					}
					bar.Add(1)
				}
			}
		}()

	}

	// 投放任务
	go func() {
		defer close(jobs)
		for _, group := range ipGroups {
			for _, ip := range group {
				select {
				case <-ctx.Done(): // 如果任务取消，立即停止投放，释放协程
					return
				case jobs <- ip:
				}
			}
		}
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
	// 停止旋转图标
	//fmt.Print("\r") // 结束后清除掉那个图标
	close(resultsChan)
	<-done // 等待结果切片填充完毕

	// 按延迟排序
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].RawLatency < finalResults[j].RawLatency
	})

	return finalResults
}

// 启动测速
func RunDeepTest(ctx context.Context, outCount int, domain string, minSpeed float64, finalResults []FinalResult, l Logger) []FinalResult {
	var finalSorted []FinalResult
	outResults := 0

	//limit := min(len(finalResults), outCount*2)

	//for i := 0; i < limit; i++ {
	for i := 0; i < len(finalResults); i++ {
		select {
		case <-ctx.Done():
			l.WriteLog("🛑 深度测速已手动停止")
			return finalSorted // 立即返回已经得到的结果
		default:
		}
		bestIP := finalResults[i].IP

		if checkWSAvailability(bestIP, domain) {
			l.WriteLog(fmt.Sprintf("⚠️ [%s] WS 连接被封锁，跳过测速\n", bestIP))
			continue
		}

		speed, err := TestSpeed(ctx, bestIP, domain, 5*time.Second, l)

		if err != nil {
			if ctx.Err() != nil {
				return finalSorted
			}
			l.WriteLog(fmt.Sprintf("下载测速异常: %v [%s]\n", err, bestIP))
			continue
		} else if speed < minSpeed {
			l.WriteLog(fmt.Sprintf("速率过低: [%s] 速度: %.2f MB/s\n", bestIP, speed))
			continue
		} else {
			l.WriteLog(fmt.Sprintf("🚀 [%s] 速度: %.2f MB/s\n", bestIP, speed))
		}

		finalSorted = append(finalSorted, FinalResult{
			IP:          bestIP,
			DownloadMBs: speed,                   // 对应结构体中的 DownloadMBs 字段
			Latency:     finalResults[i].Latency, // 别忘了把第一轮测得的延迟也带过来，方便存入 CSV
			CreatedAt:   time.Now(),              // 记录这一刻的时间
		})

		outResults++
		if outResults == outCount {
			return finalSorted
		}
	}

	// 按速度再次排序
	sort.Slice(finalSorted, func(i, j int) bool {
		return finalSorted[i].DownloadMBs > finalSorted[j].DownloadMBs
	})

	return finalSorted
}

// 模拟检测逻辑
func checkWSAvailability(ip string, host string) bool {
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{ServerName: host},
		NetDial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, ip+":443", 3*time.Second)
		},
	}

	// 尝试建立连接
	_, resp, err := dialer.Dial("wss://"+host+"/your-path", nil)
	if err != nil {
		if resp != nil && resp.StatusCode == 403 {
			// 明确捕获 403，说明此 IP 对当前 Host 封锁了 WS
			return false
		}
		return false
	}
	return true
}
