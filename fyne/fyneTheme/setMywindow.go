package fyneTheme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func SetMyWindow(window fyne.Window) {
	// --- 参数区：使用 Form 布局更整齐 ---
	threadEntry := widget.NewEntry();  threadEntry.SetText("100")
	testNumEntry := widget.NewEntry(); testNumEntry.SetText("500")
	latencyEntry := widget.NewEntry(); latencyEntry.SetText("200")
	speedEntry   := widget.NewEntry(); speedEntry.SetText("5")

	// 将参数排成一行（横向容器）
	inputRow := container.NewGridWithColumns(4,
		container.NewVBox(widget.NewLabel("线程数"), threadEntry),
		container.NewVBox(widget.NewLabel("抽样数"), testNumEntry),
		container.NewVBox(widget.NewLabel("延时限制"), latencyEntry),
		container.NewVBox(widget.NewLabel("下载速度"), speedEntry),
	)

	// --- 状态与按钮区 ---
	statusLabel := widget.NewLabel("状态: 待机")
	
	startBtn := widget.NewButtonWithIcon("开始扫描", theme.MediaPlayIcon(), func() {
		statusLabel.SetText("状态: 运行中...")
	})
	startBtn.Importance = widget.HighImportance // 蓝色高亮

	stopBtn := widget.NewButtonWithIcon("停止", theme.MediaStopIcon(), func() {})
    stopBtn.Importance = widget.DangerImportance // 红色高亮（较新版本 Fyne 支持）
	stopBtn.Disable()

	clearBtn := widget.NewButton("清空", func() {})

	controls := container.NewVBox(
		inputRow,
		container.NewHBox(statusLabel, layout.NewSpacer(), startBtn, stopBtn, clearBtn),
	)

	// --- 日志区：复刻 #log-container ---
	logDisplay := widget.NewMultiLineEntry()
	logDisplay.Disabled()
	logDisplay.TextStyle = fyne.TextStyle{Monospace: true}
    
    // 模拟 CSS 的背景色和 Padding
	logBg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 255})
	logContainer := container.NewStack(logBg, logDisplay)

	// 最终布局
	mainLayout := container.NewBorder(
		controls,     // Top
		nil,          // Bottom
		nil, nil,     // Left, Right
		logContainer, // Center (填充剩余空间)
	)

	window.SetContent(mainLayout)
	window.Resize(fyne.NewSize(900, 600))
	window.ShowAndRun()
}
