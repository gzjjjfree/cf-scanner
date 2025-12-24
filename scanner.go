package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

// 结构体定义，用于 JSON 和 CSV 导出
type FinalResult struct {
	IP          string    `json:"address"`
	Latency     string    `json:"-"` // 用于展示和 CSV 的字符串
	DownloadMBs float64   `json:"-"` // 下载速度
	RawLatency  int64     `json:"-"` // 内部排序用的数值 (ms)
	isSuccess   bool      `json:"-"`
	CreatedAt   time.Time `json:"-"` // 新增：记录测试时间
}

// ScanIP 对指定 IP 进行探测
func ScanIP(ip string, domain string, timeout time.Duration, latency int64) FinalResult {
	// 提取纯域名用于 SNI
	sni := domain
	if strings.HasPrefix(sni, "http") {
		u, _ := url.Parse(sni)
		sni = u.Host
	} else if idx := strings.Index(sni, "/"); idx != -1 {
		sni = sni[:idx]
	}

	network := "tcp"
	if strings.Contains(ip, ":") {
		network = "tcp6"
	}

	start := time.Now()

	// 1. TCP 拨号测试
	conn, err := net.DialTimeout(network, net.JoinHostPort(ip, "443"), timeout)
	if err != nil {
		// 如果失败，返回 IP，但标记 isSuccess 为 false
		return FinalResult{IP: ip, isSuccess: false}
	}
	defer conn.Close()

	// 2. TLS 握手测试
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: true,
	})

	tlsConn.SetDeadline(time.Now().Add(timeout))
	err = tlsConn.Handshake()
	if err != nil {
		return FinalResult{IP: ip, isSuccess: false}
	}

	// 计算延迟
	duration := time.Since(start)

	// 延迟超过 latency 不返回
	if duration.Milliseconds() > latency {
		return FinalResult{IP: ip, isSuccess: false}
	}

	return FinalResult{
		IP:         ip,
		Latency:    fmt.Sprintf("%dms", duration.Milliseconds()),
		RawLatency: duration.Milliseconds(), // 存入纯数字
		isSuccess:  true,
	}
}

// SpeedResult 存储测速结果
type SpeedResult struct {
	IP    string
	Speed float64 // 单位: Mbps
}

// TestSpeed 对指定 IP 进行下载测速
func TestSpeed(ip string, domain string, timeout time.Duration) (float64, error) {
	// 1. 修正 domain 参数
	// 去掉 https:// 或 http:// 协议头
	cleanDomain := strings.TrimPrefix(domain, "https://")
	cleanDomain = strings.TrimPrefix(cleanDomain, "http://")

	// 截取第一个 "/" 之前的部分（即获取纯域名/主机名）
	if idx := strings.Index(cleanDomain, "/"); idx != -1 {
		cleanDomain = cleanDomain[:idx]
	}

	// 2. 创建一个自定义的传输层
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// 这一行非常重要：跳过证书过期、域名不匹配等所有校验
			InsecureSkipVerify: true,
			// 记得带上 SNI
			ServerName: cleanDomain,
		},
		// 核心逻辑：强制将所有连接指向指定的测速 IP
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip, "443"))
		},
		ForceAttemptHTTP2: false, // 开启 HTTP/2 提高性能
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout + 5*time.Second, // 测速超时稍长一点
	}

	// 3. 构造下载请求
	// 建议在服务器上放一个 10MB 的测试文件，如果没有，可以暂时请求主页
	url := fmt.Sprintf("https://%s", domain)
	req, _ := http.NewRequest("GET", url, nil)
	// 必须手动指定 Host，这要和你的域名完全一致
	req.Host = cleanDomain
	// 补齐模拟浏览器的头部
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	// 使用 Context 实现“采样时间一到立即切断”
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		// 情况 A：连接阶段就超时了，或者网络根本不通
		// 此时 resp 是 nil，直接返回 0，不需要 Close
		return 0, err
	}

	defer resp.Body.Close()

	// 5. 设置一个标记，用于判断是否已经成功接收到首字节
	firstByteReceived := make(chan struct{})

	// 6. 启动定时器监控首字节
	go func() {
		select {
		case <-firstByteReceived:
			// 正常接收到首字节，协程安全退出
			return
		case <-time.After(2 * time.Second):
			// 2秒内没收到首字节，强行关闭，触发 Read 报错
			fmt.Printf("\n[IP: %s] 首字节超时，跳过\n", ip)
			resp.Body.Close()
		}
	}()

	// 7. 读取内容并计算字节数
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(" \t"),
		progressbar.OptionSetWriter(os.Stdout), // 改用 Stdout 试试
		progressbar.OptionShowBytes(false),     // 关闭字节显示
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetPredictTime(false), // 关闭剩余时间预测
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionClearOnFinish(), // 完成后清理，保持界面整洁
	)

	// 8. 核心：在规定时间内读取数据
	// 我们手动处理读取过程，计算读取了多少字节
	var downloadedBytes int64
	buffer := make([]byte, 64*1024) // 32KB 缓冲区
	// 记录真正开始下载的时间（排除握手时间）
	var downloadStart time.Time
	firstByte := true

	for {
		n, readErr := resp.Body.Read(buffer)
		if firstByte && n > 0 {
			close(firstByteReceived)   // 核心：通知上面的协程，我们拿到数据了！
			downloadStart = time.Now() // 记录收到第一个字节的时间
			firstByte = false
		}

		if n > 0 {
			downloadedBytes += int64(n)
			bar.Add(n)
		}

		if readErr != nil {
			// 情况 B：读取过程中时间到了（context deadline exceeded）
			// 这是正常的，我们跳出循环去计算已经下载了多少
			//fmt.Println(readErr.Error())
			if readErr == io.EOF || strings.Contains(readErr.Error(), "context deadline exceeded") {
				break
			}
			// 如果是其他真实的读取错误，才返回 error
			return 0, readErr
		}
	}

	// 9. 测速完成后，清理掉那个斜杠，保持界面整洁
	bar.Describe("Done")
	bar.Finish()

	// 使用真正下载所耗费的时间来计算，这样结果最准
	actualDuration := time.Since(downloadStart).Seconds()
	fmt.Printf("下载耗费时间: %.2f 秒 ", actualDuration)
	if actualDuration <= 0 || downloadedBytes == 0 {
		return 0, fmt.Errorf("测速数据不足")
	}

	// 10. 公式：字节 * 8 / 1024 / 1024 / 秒
	mbps := (float64(downloadedBytes) * 8) / (1024 * 1024) / actualDuration
	return mbps, nil
}
