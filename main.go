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
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	videoRoot  string
	mergeMutex sync.Mutex
)

func init() {
	// 优先读取环境变量，默认为 /root/Videos
	videoRoot = os.Getenv("VIDEO_ROOT")
	if videoRoot == "" {
		videoRoot = "/root/Videos"
	}
}

type MergeRequest struct {
	Files []string `json:"files"`
}

// VideoInfo 用于返回给前端的详细信息
type VideoInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"` // Unix timestamp
}

func scanVideos(w http.ResponseWriter, r *http.Request) {
	var files []VideoInfo
	err := filepath.Walk(videoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		// 支持所有常见直播录像格式
		if ext == ".mp4" || ext == ".flv" || ext == ".ts" || ext == ".mkv" {
			rel, err := filepath.Rel(videoRoot, path)
			if err == nil {
				// 统一转为正斜杠，确保前端兼容性
				safePath := filepath.ToSlash(rel)
				files = append(files, VideoInfo{
					Path:    safePath,
					Size:    info.Size(),
					ModTime: info.ModTime().Unix(),
				})
			}
		}
		return nil
	})

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 按修改时间倒序排列 (最新的在前)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func mergeVideos(w http.ResponseWriter, r *http.Request) {
	if !mergeMutex.TryLock() {
		http.Error(w, "⚠️ 系统忙：已有合并任务正在进行", http.StatusTooManyRequests)
		return
	}
	defer mergeMutex.Unlock()

	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效请求", 400)
		return
	}

	// 1. 创建列表文件
	listFile, err := os.CreateTemp("", "ffmpeg_list_*.txt")
	if err != nil {
		http.Error(w, "无法创建临时文件", 500)
		return
	}
	defer os.Remove(listFile.Name())

	log.Println("--- 开始新的合并任务 ---")

	// 2. 写入文件路径
	for _, file := range req.Files {
		// 路径安全清洗
		localPath := filepath.FromSlash(file)
		absPath := filepath.Join(videoRoot, localPath)

		if !strings.HasPrefix(filepath.Clean(absPath), videoRoot) {
			http.Error(w, "非法路径", 403)
			return
		}

		// 转为 FFmpeg 兼容格式 (正斜杠)
		safePath := filepath.ToSlash(filepath.Clean(absPath))
		io.WriteString(listFile, fmt.Sprintf("file '%s'\n", safePath))
	}
	listFile.Close()

	// 3. 生成输出路径
	firstFile := req.Files[0]
	ext := filepath.Ext(firstFile)
	if ext == "" {
		ext = ".mp4"
	}

	baseName := strings.TrimSuffix(filepath.Base(firstFile), ext)
	outputName := fmt.Sprintf("%s_merged_%d%s", baseName, time.Now().Unix(), ext)

	// 输出到同级目录
	outputDir := filepath.Dir(filepath.Join(videoRoot, filepath.FromSlash(firstFile)))
	outputPath := filepath.Join(outputDir, outputName)

	// 4. 构建 FFmpeg 命令 (包含 AAC 修复过滤器)
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listFile.Name(),
		"-c", "copy",              // 视频流无损复制
		"-bsf:a", "aac_adtstoasc", // 【关键】修复 AAC 音频流时间戳
		"-y",                      // 覆盖输出
		outputPath,
	}

	log.Printf("执行命令: ffmpeg %v", args)
	cmd := exec.Command("ffmpeg", args...)

	// 捕获输出以便调试
	output, err := cmd.CombinedOutput()
	log.Printf("FFmpeg 输出:\n%s", string(output))

	if err != nil {
		log.Printf("❌ 合并失败: %v", err)
		http.Error(w, fmt.Sprintf("合并失败: %v", err), 500)
		return
	}

	log.Printf("✅ 合并成功: %s", outputName)
	w.Write([]byte(fmt.Sprintf("✅ 合并成功！\n文件: %s", outputName)))
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/api/videos", scanVideos)
	http.HandleFunc("/api/merge", mergeVideos)

	log.Printf("Docker 服务启动 - 监听 :8082")
	log.Printf("监控根目录: %s", videoRoot)
	log.Fatal(http.ListenAndServe(":8082", nil))
}