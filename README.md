# Cloudflare SpeedTest & V2Ray Optimizer

一个基于 Go 语言开发的 Cloudflare 优选 IP 扫描工具。

# cf-scanner

![Build Status](https://img.shields.io/github/actions/workflow/status/gzjjjfree/cf-scanner/release.yml?branch=main&style=flat-square)

![Latest Release](https://img.shields.io/github/v/release/gzjjjfree/cf-scanner?style=flat-square&color=blue)

![License](https://img.shields.io/github/license/gzjjjfree/cf-scanner?style=flat-square)

![Downloads](https://img.shields.io/github/downloads/gzjjjfree/cf-scanner/total?style=flat-square&color=orange)

## ✨ 特性
- **多阶段扫描**：支持 IP 段各段随机抽样，兼顾效率与覆盖面。
- **自动适配**：直接输出 `result.json` 供 V2Ray 客户端加载 IP 池。
- **实时反馈**：带动态旋转图标的进度条，展示详细测速耗时。
- **测速效果**：不测试丢包率，注重延迟与下载速度，实测效果显著。

# 📖 使用指南 (Usage Guide)

本工具旨在帮助用户在海量的 Cloudflare IP 中精准筛选出适合本地网络环境的优质节点。

---

### 1. 准备工作在运行程序前，请确保当前目录下存在一个 ip.txt 文件。
* **格式**：每行一个 CIDR 格式的 IP 段（例如 104.16.0.0/12）或单个 IP 地址。
* **推荐**：您可以从 [Cloudflare 官方 IPv4 地址列表](https://www.cloudflare.com/ips-v4) 获取最新的网段。
 
### 2. 常用运行命令

您可以根据需求调整扫描强度：

* **标准扫描**（推荐，用于日常优选，默认测试 Cloudflare 官方网站）：
  ```bash
  ./cf-scanner
  ```

* **命令行参数扫描**
* 请运行 -h 查看具体用法
  ```bash
  ./cf-scanner -h
  ```

* **result.json 用法**
* 配合 cloudflare-vless-worker/worker.js 及 v5-result 使用 
* 具体用法查看上两个项目