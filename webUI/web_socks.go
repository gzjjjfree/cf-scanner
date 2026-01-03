package webUI

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gzjjjfree/cf-scanner/scanner"
)

// 定义发送给前端的消息结构
type WSMessage struct {
	Type string `json:"type"` // "log" 或 "status"
	Data any    `json:"data"` // 日志内容 或 状态对象
}

// --- WebSocket 相关配置 ---
var (
	// 升级器：将 HTTP 协议提升为 WebSocket
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源，方便开发测试
		},
	}

	// 客户端管理：支持多个浏览器窗口同时接收日志
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

// --- WebSocket 处理器 ---
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket 升级失败: %v\n", err)
		return
	}

	// 注册新客户端
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	// 保持连接，处理断开情况
	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	status.waitStop = false
	// --- 关键：监听前端发来的消息 ---
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var cmd WSMessage
		if err := json.Unmarshal(message, &cmd); err != nil {
			continue // 忽略非法格式
		}

		// 根据 Type 分发逻辑
		switch cmd.Type {
		case "start":
			// 1. 首先断言 cmd.Data 是一个 map
			rawParams, ok := cmd.Data.(map[string]any)
			if !ok {
				WriteLog("❌ 错误：无效的参数格式\n")
				status.IsRunning = false
				broadcastStatus(false)
				continue
			}

			// 2. 创建一个真正的 map[string]string
			cleanParams := make(map[string]string)
			for k, v := range rawParams {
				// 将 any 转换为 string（处理字符串、数字、布尔等）
				cleanParams[k] = fmt.Sprintf("%v", v)
			}
			// 你可以从 cmd.Type 中提取前端传来的参数
			go startScanWorkflow(cleanParams)
		case "stop":
			if scanner.CancelScan != nil {
				scanner.CancelScan()
				status.waitStop = true
				WriteLog("\n🛑 正在通过 WebSocket 指令停止任务...\n")
			}
		}
	}
}

// 启动前处理参数
func startScanWorkflow(params map[string]string) {
	statusMutex.Lock()
	if status.IsRunning {
		statusMutex.Unlock()
		return
	}
	var scanCtx context.Context
	scanCtx, scanner.CancelScan = context.WithCancel(context.Background())
	status.IsRunning = true
	statusMutex.Unlock()

	defer func() {
		statusMutex.Lock()
		status.IsRunning = false
		statusMutex.Unlock()
	}()

	// 赋予默认值
	var config scanner.ScanConfig

	// 转换 Threads (string -> int)
	if val, ok := params["threads"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 1 && i < 101) {
			config.NThreads = i
		}
	}

	// 转换 MinSpeed (string -> float64)
	if val, ok := params["min_speed"]; ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil && (f > 0.1 && f < 21) {
			config.MinSpeed = f
		}
	}

	// 转换 MinLatency (string -> int)
	if val, ok := params["min_latency"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 10 && i < 1001) {
			config.MinLatency = int64(i)
		}
	}

	// 转换 FinalCount (string -> int)
	if val, ok := params["final_count"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 1 && i < 501) {
			config.FinalCount = i
		}
	}

	// 转换 TestNum (string -> int)
	if val, ok := params["test_num"]; ok {
		if i, err := strconv.Atoi(val); err == nil && (i > 10 && i < 2001) {
			config.TestNum = i
		}
	}

	config.Check()

	runScannerLogic(scanCtx, config)
}

// 发送状态更新（在扫描开始和结束时调用）
func broadcastStatus(isScanning bool) {
	msg := WSMessage{
		Type: "status",
		Data: map[string]bool{"is_scanning": isScanning},
	}
	payload, _ := json.Marshal(msg)

	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		client.WriteMessage(websocket.TextMessage, payload)
	}
}
