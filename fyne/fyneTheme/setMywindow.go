package fyneTheme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gzjjjfree/cf-scanner/fyne/goScan"
	png "github.com/gzjjjfree/cf-scanner/fyne/png/pnggo"
	"github.com/gzjjjfree/cf-scanner/scanner"
)

type tightTheme struct {
	parent fyne.Theme
}

// 覆盖字体：确保中文不乱码
func (m tightTheme) Font(style fyne.TextStyle) fyne.Resource {
	// 直接使用父级的字体逻辑
	return m.parent.Font(style)
}

// 覆盖尺寸：保持紧凑
func (m tightTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding || name == theme.SizeNameInnerPadding {
		return 1
	}
	if name == theme.SizeNameText {
		return 20
	}
	return m.parent.Size(name)
}

// 覆盖颜色
func (m tightTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// 只有在这个覆盖区内的文字才强制纯白
	if name == theme.ColorNameForeground ||
		name == theme.ColorNameDisabled ||
		name == theme.ColorNamePlaceHolder {
		return color.White
	}

	// 背景色让它透明，这样才能露出底下的 logBg 矩形
	if name == theme.ColorNameBackground {
		return color.Transparent
	}
	// 其余颜色（如背景）去问父级（即你的 chineseTheme）
	return m.parent.Color(name, variant)
}

// 必须实现的其余接口方法
func (m tightTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return m.parent.Icon(name) }

func SetMyWindow(window fyne.Window, a fyne.App) {
	// 获取你在 main 里 SetTheme 的那个主题 (chineseTheme)
	mainTheme := a.Settings().Theme()

	// --- 参数区：使用 Form 布局更整齐 ---
	threadEntry := widget.NewEntry()
	threadEntry.SetText("100")
	testNumEntry := widget.NewEntry()
	testNumEntry.SetText("500")
	latencyEntry := widget.NewEntry()
	latencyEntry.SetText("200")
	speedEntry := widget.NewEntry()
	speedEntry.SetText("5")
	countEntry := widget.NewEntry()
	countEntry.SetText("10")

	imgEnBtnSize := fyne.NewSize(200, 30)
	threadBtn := NewImageButton(png.ResourceThreadsPng, imgEnBtnSize, func() {})
	testNumBtn := NewImageButton(png.ResourceTestNumPng, imgEnBtnSize, func() {})
	latencyBtn := NewImageButton(png.ResourceLatencyPng, imgEnBtnSize, func() {})
	speedBtn := NewImageButton(png.ResourceSpeedPng, imgEnBtnSize, func() {})
	finalCountBtn := NewImageButton(png.ResourceFinalCountPng, imgEnBtnSize, func() {})

	// 将参数排成一行（横向容器）
	inputRow := container.NewGridWithColumns(5,
		container.NewVBox(threadBtn, threadEntry),
		container.NewVBox(testNumBtn, testNumEntry),
		container.NewVBox(latencyBtn, latencyEntry),
		container.NewVBox(speedBtn, speedEntry),
		container.NewVBox(finalCountBtn, countEntry),
	)

	imgBtnSize := fyne.NewSize(100, 40)
	var startBtn *ImageButton
	var stopBtn *ImageButton

	// 定义 StringList 绑定
	logDataList := binding.NewStringList()

	// 创建 List 组件
	logDisplay := widget.NewListWithData(
		logDataList,
		func() fyne.CanvasObject {
			// 使用单行、无边距的 Label
			label := widget.NewLabel("")
			label.TextStyle = fyne.TextStyle{Bold: true}
			return label
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			// 将数据绑定到每一行的 Label 上
			o.(*widget.Label).Bind(i.(binding.String))
		},
	)
	logWithTheme := container.NewThemeOverride(
		logDisplay,
		tightTheme{parent: mainTheme}, // 将全局主题传进去作为父级
	)

	// 把日志框放进滚动容器，这样才能滚动
	scrollContainer := container.NewVScroll(logWithTheme)
	scrollContainer.SetMinSize(fyne.NewSize(0, 300)) // 设置最小高度

	// 模拟 CSS 的背景色和 Padding
	logBg := canvas.NewRectangle(color.NRGBA{R: 15, G: 15, B: 15, A: 255})

	logDataList.Append("等待扫描开始...")
	
	logDisplay.HideSeparators = true // 关键：隐藏白条
	logDisplay.OnSelected = func(id widget.ListItemID) {
		logDisplay.Unselect(id) // 只要被选中就立刻取消选中，防止变色
	}

	// 核心逻辑：开启一个后台协程，永久监听通道
	go func() {
		for msg := range goScan.LogChan {
			// 取得信息行的换行符个数
			before, behide := countNewlines(msg, "\n")
			beforer, behider := countNewlines(msg, "\r")

			// 行前的换行符，每个 \n 增加一行 logDataList
			for range before {
				logDataList.Append(" ")
			}
			msg = strings.NewReplacer("\n", "", "\r", "").Replace(msg)
			lastIdx := logDataList.Length() - 1
			if lastIdx >= 0 { // 已有行数才进行更新行操作
				if beforer > 0 || behider > 0 { // 有换行符为 \r 的为进度条，对最后一行进行更新
					logDataList.SetValue(lastIdx, msg)
				} else { // 没有换行符时，追加到最后一行
					data, _ := logDataList.GetValue(lastIdx)
					logDataList.SetValue(lastIdx, fmt.Sprint(data, msg))
				}

				// // 行后的换行符，每个 \n 增加一行 logDataList
				for range behide {
					logDataList.Append(" ")
				}
				fyne.Do(func() { logDisplay.ScrollToBottom() })
				continue
			} else { // 没有行数时，直接增加一行
				logDataList.Append(msg)
			}

			for range behide {
				logDataList.Append(" ")
			}
			fyne.Do(func() { logDisplay.ScrollToBottom() })
		}
	}()

	// --- 状态与按钮区 ---
	statusLabel := widget.NewLabel("状态: 待机")

	// 开始按钮
	startBtn = NewImageButton(png.ResourceStartPng, imgBtnSize, func() {
		statusLabel.SetText("状态: 运行中...")

		// 设置按钮的变化
		startBtn.Disable(png.ResourceStartdPng)		
		stopBtn.Enable(png.ResourceStopPng)

		// 读取输入框中的字符串并转换类型
		threads, _ := strconv.Atoi(threadEntry.Text)
		testNum, _ := strconv.Atoi(testNumEntry.Text)
		latency, _ := strconv.Atoi(latencyEntry.Text)
		speed, _ := strconv.ParseFloat(speedEntry.Text, 64)
		// 转到扫描逻辑
		goScan.ReceivingParameters(threads, testNum, int64(latency), speed)

		logDataList.Set(nil) // 清空旧日志

		// 开启协程监听
		go func() {
			for {
				// 阻塞等待信号，接收到 false 表示已停止了扫描
				if !<-goScan.FinishChan {
					fyne.Do(func() {
						statusLabel.SetText("状态: 待机")
						startBtn.Enable(png.ResourceStartPng)
						stopBtn.Disable(png.ResourceStopdPng)
					})
				}
			}
		}()
	})

	// 停止按钮
	stopBtn = NewImageButton(png.ResourceStopdPng, imgBtnSize, func() {
		statusLabel.SetText("状态: 停止中...")
		
		stopBtn.Disable(png.ResourceStopdPng)
		scanner.CancelScan()
		goScan.Status.WaitStop = true
	})

	clearBtn := NewImageButton(png.ResourceClearPng, imgBtnSize, func() {
		// 重置 UI 绑定数据
		logDataList.Set(nil)

		// 可选：重置状态信息
		statusLabel.SetText("状态: 已清空")
	})

	controls := container.NewVBox(
		inputRow,
		container.NewHBox(statusLabel, layout.NewSpacer(), layout.NewSpacer(), startBtn, layout.NewSpacer(), stopBtn, layout.NewSpacer(), clearBtn),
	)
	logContainer := container.NewStack(logBg, scrollContainer)

	// 最终布局
	mainLayout := container.NewBorder(
		controls, // Top
		nil,      // Bottom
		nil, nil, // Left, Right
		logContainer, // Center (填充剩余空间)
	)

	window.SetContent(mainLayout)
	window.Resize(fyne.NewSize(900, 600))
	window.ShowAndRun()
}

func countNewlines(s string, find string) (leading int, trailing int) {
	temp := s
	// 统计开头的 \n
	for strings.HasPrefix(temp, find) {
		leading++
		temp = temp[1:] // 移动指针，切掉第一个字符
	}

	temp = s
	// 统计结尾的 \n
	for strings.HasSuffix(temp, find) {
		trailing++
		temp = temp[:len(temp)-1] // 移动指针，切掉最后一个字符
	}

	return leading, trailing
}
