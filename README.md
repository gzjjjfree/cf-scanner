# Cloudflare SpeedTest & V2Ray Optimizer

一个基于 Go 语言开发的 Cloudflare 优选 IP 扫描工具。

# cf-scanner

![Build Status](https://img.shields.io/github/actions/workflow/status/gzjjjfree/cf-scanner/release.yml?branch=main&style=flat-square)

![Latest Release](https://img.shields.io/github/v/release/gzjjjfree/cf-scanner?style=flat-square&color=blue)

![License](https://img.shields.io/github/license/gzjjjfree/cf-scanner?style=flat-square)

![Downloads](https://img.shields.io/github/downloads/gzjjjfree/cf-scanner/total?style=flat-square&color=orange)

## ✨ 特性
- **多阶段扫描**：支持 IP 段随机抽样，兼顾效率与覆盖面。
- **自动适配**：直接输出 `result.json` 供 V2Ray 客户端加载 IP 池。
- **实时反馈**：带动态旋转图标的进度条，展示详细测速耗时。

## 🚀 快速开始
1. 下载对应平台的 [Releases](你的链接) 版本。
2. 准备 `ip.txt`（每行一个 IP 段）[ip.txt](https://www.cloudflare.com/ips-v4)。
3. 运行扫描：
   ```bash
   ./cf-scanner -d 你网站上测速的文件路径如：www.speed.com/10mb.bin