package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/schollz/progressbar/v3"
)

// 定义一个空的结构体，作为接口的载体
type LorcaLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

// 让 WebLogger 实现 WriteLog 方法
func (w LorcaLogger) WriteLog(msg string) {
	content := scanner.WSMessage{
		Type: "log",
		Data: msg,
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		fmt.Printf("JSON 序列化失败: %v\n", err)
		return
	}

	ui.Eval(fmt.Sprintf(`window.handleScanUpdate(%s)`, string(jsonData)))
}

func (w LorcaLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w LorcaLogger) GetColorCodes() bool {
	return false
}

var BridgeLogger = LorcaLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}
