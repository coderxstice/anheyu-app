/*
 * @Description: 代理处理器，用于处理外部资源下载
 * @Author: 安知鱼
 * @Date: 2025-01-20 10:00:00
 * @LastEditTime: 2025-08-31 12:25:14
 * @LastEditors: 安知鱼
 */
package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"path" // <-- 优化：导入 path 包
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 代理处理器
type ProxyHandler struct{}

// NewHandler 创建代理处理器
func NewHandler() *ProxyHandler {
	return &ProxyHandler{}
}

// HandleDownload 处理外部文件下载代理
// @Summary      代理下载
// @Description  代理下载外部资源（主要用于图片）
// @Tags         代理服务
// @Produce      octet-stream
// @Param        url  query  string  true  "目标URL"
// @Success      200  {file}    file  "文件内容"
// @Failure      400  {object}  object{error=string}  "参数错误"
// @Failure      502  {object}  object{error=string}  "目标服务器错误"
// @Failure      500  {object}  object{error=string}  "代理失败"
// @Router       /proxy/download [get]
func (h *ProxyHandler) HandleDownload(c *gin.Context) {
	// 获取要下载的URL
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少url参数"})
		return
	}

	// 验证URL格式
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的URL格式"})
		return
	}

	// 只允许http和https协议
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持http和https协议"})
		return
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建请求失败"})
		return
	}

	// 设置请求头，模拟真实浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// ★★★★★ 关键修复点 ★★★★★
	// 不要请求压缩数据，因为我们不在代理中处理解压。
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br")

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "目标服务器返回错误: " + resp.Status})
		return
	}

	// 检查Content-Type，如果服务器未提供，则尝试设为通用二进制流
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // 提供一个默认值
	}

	// 为了安全，仍然可以检查是否是预期的图片类型
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "目标不是图片文件: " + contentType})
		return
	}

	// 从原始响应中复制必要的头信息到我们的响应中
	c.Header("Content-Type", contentType)
	if resp.Header.Get("Content-Length") != "" {
		c.Header("Content-Length", resp.Header.Get("Content-Length"))
	}

	// 设置我们自己的头信息
	c.Header("Content-Disposition", "attachment; filename="+getFileName(targetURL))
	c.Header("Cache-Control", "no-cache")
	c.Header("Access-Control-Allow-Origin", "*")

	// 流式传输文件内容
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		// 优化：此时HTTP头已发送，不能再写入JSON。只能在服务端记录日志。
		log.Printf("向客户端传输文件时发生错误: %v", err)
	}
}

// HandleTest 测试代理下载功能
// @Summary      测试代理服务
// @Description  测试代理下载服务是否正常运行
// @Tags         代理服务
// @Produce      json
// @Success      200  {object}  object{message=string,timestamp=int,status=string}  "服务正常"
// @Router       /proxy/test [get]
func (h *ProxyHandler) HandleTest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":   "代理下载服务正常运行",
		"timestamp": time.Now().Unix(),
		"status":    "ok",
	})
}

// getFileName 从URL中提取文件名
func getFileName(urlStr string) string {
	// 优化：使用标准库 path.Base 来获取文件名，更健壮
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "download"
	}

	filename := path.Base(parsedURL.Path)
	if filename == "." || filename == "/" {
		return "download"
	}

	return filename
}
