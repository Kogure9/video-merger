package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	videoRoot  string
	mergeMutex sync.Mutex // 全局互斥锁
)

func init() {
	// 从环境变量读取路径，默认为 /root/Videos
	videoRoot = os.Getenv("VIDEO_ROOT")
	if videoRoot == "" {
		videoRoot = "/root/Videos"
	}
}

// MergeRequest 定义请求结构
type MergeRequest struct {
	Files []string `json:"files"`
}

// scanVideos 扫描视频文件
func scanVideos(w http.ResponseWriter, r *http.Request) {
	var files []string
	err := filepath.Walk(videoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("访问出错 %s: %v", path, err)
			return nil // 跳过错误文件
		}
		if info.IsDir() {
			return nil
		}

		// --- 优化点：支持多格式 ---
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext == ".mp4" || ext == ".flv" || ext == ".ts" || ext == ".mkv" {
			// 获取相对路径
			relativePath, err := filepath.Rel(videoRoot, path)
			if err == nil {
				// 统一转为正斜杠，方便前端处理
				files = append(files, filepath.ToSlash(relativePath))
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, "扫描失败: "+err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// mergeVideos 执行合并
func mergeVideos(w http.ResponseWriter, r *http.Request) {
	// --- 优化点：并发锁 ---
	if !mergeMutex.TryLock() {
		http.Error(w, "⚠️ 系统忙：已有合并任务正在进行，请稍后再试。", http.StatusTooManyRequests)
		return
	}
	defer mergeMutex.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的 JSON", 400)
		return
	}

	if len(req.Files) < 2 {
		http.Error(w, "请至少选择两个文件", 400)
		return
	}

	// 1. 创建 FFmpeg 列表文件
	listFile, err := os.CreateTemp("", "ffmpeg-list-*.txt")
	if err != nil {
		http.Error(w, "无法创建临时文件", 500)
		return
	}
	defer os.Remove(listFile.Name())

	// 2. 写入文件路径
	for _, file := range req.Files {
		// 防止路径遍历攻击
		localPath := filepath.FromSlash(file)
		absPath := filepath.Join(videoRoot, localPath)
		
		// 简单的安全检查
		if !strings.HasPrefix(filepath.Clean(absPath), videoRoot) {
			http.Error(w, "非法路径", 403)
			return
		}

		// 转为 FFmpeg 需要的格式
		safePath := filepath.ToSlash(filepath.Clean(absPath))
		io.WriteString(listFile, fmt.Sprintf("file '%s'\n", safePath))
	}
	listFile.Close()

	// 3. 生成输出路径
	firstFile := req.Files[0]
	ext := filepath.Ext(firstFile)
	if ext == "" { ext = ".mp4" }
	
	// 文件名: 原名_merged_时间戳.后缀
	baseName := strings.TrimSuffix(filepath.Base(firstFile), ext)
	outputName := fmt.Sprintf("%s_merged_%d%s", baseName, time.Now().Unix(), ext)
	
	// 输出到同级目录
	outputDir := filepath.Dir(filepath.Join(videoRoot, filepath.FromSlash(firstFile)))
	outputPath := filepath.Join(outputDir, outputName)

	// 4. 调用 FFmpeg
	log.Printf("开始合并: %s", outputName)
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFile.Name(), "-c", "copy", outputPath)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("合并失败: %s", string(output))
		http.Error(w, fmt.Sprintf("合并失败:\n%s", string(output)), 500)
		return
	}

	log.Printf("合并成功: %s", outputPath)
	w.Write([]byte(fmt.Sprintf("✅ 合并成功！\n文件: %s", outputName)))
}

func main() {
	// 静态文件服务
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	
	// API 路由
	http.HandleFunc("/api/videos", scanVideos)
	http.HandleFunc("/api/merge", mergeVideos)

	log.Printf("服务启动于 :8082 | 根目录: %s", videoRoot)
	log.Fatal(http.ListenAndServe(":8082", nil))
}