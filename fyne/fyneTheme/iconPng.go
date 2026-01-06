package fyneTheme

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// 定义图片按钮结构体
type ImageButton struct {
	widget.BaseWidget
	img      *canvas.Image
	OnTapped func()
	disabled bool // 内部状态锁
}

// 实现 MinSize，确保布局管理器尊重图片大小
func (b *ImageButton) MinSize() fyne.Size {
	return b.img.MinSize()
}

// 渲染组件
func (b *ImageButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.img)
}

// 处理点击事件
func (b *ImageButton) Tapped(_ *fyne.PointEvent) {
	if b.disabled {
		return // 如果已禁用，直接拦截，不执行逻辑
	}
	if b.OnTapped != nil {
		b.OnTapped()
	}
}

// SetImage 用于在运行时动态更换图片资源
func (b *ImageButton) SetImage(res fyne.Resource) {
	b.img.Resource = res // 替换资源
	b.img.Refresh()      // 刷新图片对象
	b.Refresh()          // 刷新整个组件
}

// Disable 禁用按钮
func (b *ImageButton) Disable(res *fyne.StaticResource) {
	b.disabled = true
	// 可选：在此处切换为“灰色”版本的图片，给用户视觉反馈
	b.SetImage(res)
	b.img.Refresh()
	b.Refresh()
}

// Enable 启用按钮
func (b *ImageButton) Enable(res *fyne.StaticResource) {
	b.disabled = false
	b.SetImage(res)
}

// 便捷构造函数
func NewImageButton(res fyne.Resource, size fyne.Size, tapped func()) *ImageButton {
	img := canvas.NewImageFromResource(res)
	img.SetMinSize(size) // 强制设定图片显示大小
	img.FillMode = canvas.ImageFillOriginal

	b := &ImageButton{img: img, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}
