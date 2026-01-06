package goScan

import (
	"context"
	"sync"

	"github.com/gzjjjfree/cf-scanner/scanner"
	"github.com/schollz/progressbar/v3"
)

var (
	conf        scanner.ScanConfig
	LogChan     = make(chan string, 100)
	FinishChan  = make(chan bool)
	Status      ScanStatus
	StatusMutex sync.Mutex
)

// 定义一个空的结构体，作为接口的载体
type FyneLogger struct {
	Theme progressbar.Theme
	Ctx   context.Context
}

func (w FyneLogger) WriteLog(msg string) {
	LogChan <- msg
}

func (w FyneLogger) GetTheme() progressbar.Theme {
	return w.Theme
}

func (w FyneLogger) GetColorCodes() bool {
	return false
}

var BridgeLogger = FyneLogger{
	Theme: progressbar.Theme{
		Saucer:        "=",
		SaucerHead:    ">",
		SaucerPadding: " ",
		BarStart:      "[",
		BarEnd:        "]",
	},
}

type ScanStatus struct {
	isRunning bool
	WaitStop  bool
}
