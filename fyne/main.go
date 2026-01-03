package main

import (
	"fyne/fyneTheme"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
)

// 自定义主题：用于加载中文字体
type chineseTheme struct {
	font fyne.Resource
}

func (m *chineseTheme) Font(s fyne.TextStyle) fyne.Resource {
	return m.font
}

// 其余方法保持默认
func (m *chineseTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(n, v)
}
func (m *chineseTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}
func (m *chineseTheme) Size(n fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(n)
}

func main() {
	myApp := app.New()

	myApp.Settings().SetTheme(&chineseTheme{font: resourceSimheiTtf})

	myWindow := myApp.NewWindow("CF-Scanner 原生版")
	fyneTheme.SetMyWindow(myWindow)

}
