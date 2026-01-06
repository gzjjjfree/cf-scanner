package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 1. 极简中文字体主题
type simpleTheme struct {
	font fyne.Resource
}

func (m *simpleTheme) Font(s fyne.TextStyle) fyne.Resource { return m.font }
func (m *simpleTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	// 强制控制区的文字为纯黑色，增加对比度
	if n == theme.ColorNameForeground {
		//return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		return color.Black
	}
	return theme.DefaultTheme().Color(n, v)
}

func (m *simpleTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameText {
		return 20 // 稍微调大字体，从视觉上增加“粗细感”
	}
	return theme.DefaultTheme().Size(n)
}
func (m *simpleTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func main() {
	myApp := app.New()
	// 注意：请确保 resourceSimheiTtf 是你生成的资源变量名
	myApp.Settings().SetTheme(&simpleTheme{font: resourceSimheiTtf})

	win := myApp.NewWindow("原点测试 - 修正版")

	// --- [控制区] 保持默认（应该是黑字） ---
	topLabel := widget.NewLabel("控制区标签：线程数")
	topLabel.TextStyle = fyne.TextStyle{Bold: true} // 关键：强制粗体
	testBtn := widget.NewButton("开始测试", func() {})
topLabel.Refresh()
	// --- [日志区] 强行黑底白字 ---
	// 使用 canvas.Text 它是 Fyne 中最亮、最底层的文字对象
	logText := canvas.NewText("日志：🚀 扫描启动... 100% 亮度测试", color.White)
	logText.TextSize = 18
	logText.TextStyle = fyne.TextStyle{Monospace: true}

	// 黑色背景矩形
	logBg := canvas.NewRectangle(color.Black)

	// 用 Stack 把文字叠在黑色矩形上面
	// 用 Padded 留出一点边距，不至于顶格
	logArea := container.NewStack(
		logBg,
		container.NewPadded(logText),
	)

	// --- 最终组装 ---
	content := container.NewVBox(
		createHighContrastLabel("控制区标签：线程数"),
		topLabel,
		testBtn,
		container.NewGridWrap(fyne.NewSize(600, 200), logArea), // 固定日志区大小
	)
content.Refresh()
	win.SetContent(content)
	win.Resize(fyne.NewSize(700, 450))
	win.CenterOnScreen()
	win.ShowAndRun()
}
func createHighContrastLabel(text string) fyne.CanvasObject {
    l := canvas.NewText(text, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
    l.TextSize = 20
    l.TextStyle = fyne.TextStyle{Bold: true}
	l.Refresh()
    return l
}