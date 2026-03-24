#!/bin/bash

# --- 配置 ---
VIDEO_DIR="/vol1/1000/Docker/bililive-go-master/Videos"
THRESHOLD_GB=200
# 修改后的日志存放目录
LOG_DIR="/vol1/1000/程序/log"
LOG_FILE="$LOG_DIR/video_cleanup.log"

# --- 检查并创建日志目录 ---
if [ ! -d "$LOG_DIR" ]; then
    mkdir -p "$LOG_DIR"
fi

# --- 日志记录函数 ---
log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log_message "--- 自动清理任务开始 ---"

# 检查视频目录
if [ ! -d "$VIDEO_DIR" ]; then
    log_message "❌ 错误: 目录 $VIDEO_DIR 不存在。"
    exit 1
fi

# 1. 获取初始目录大小 (单位: KB)
CURRENT_SIZE_KB=$(du -skL "$VIDEO_DIR" 2>/dev/null | awk '{print $1}')

# 使用 awk 计算阈值 KB，避免使用 bc
THRESHOLD_KB=$(awk -v gb="$THRESHOLD_GB" 'BEGIN {print int(gb * 1024 * 1024)}')

# 计算当前 GB 供日志显示
CURRENT_GB=$(awk -v kb="$CURRENT_SIZE_KB" 'BEGIN {printf "%.2f", kb/1024/1024}')

log_message "目标目录: $VIDEO_DIR"
log_message "当前大小: ${CURRENT_GB} GB"
log_message "设定阈值: $THRESHOLD_GB GB"

# 2. 循环清理
while [ "$CURRENT_SIZE_KB" -gt "$THRESHOLD_KB" ]; do
    # 查找最旧的文件 (mp4, flv, ts)
    FILE_INFO=$(find "$VIDEO_DIR" -type f \( -name "*.mp4" -o -name "*.flv" -o -name "*.ts" \) -printf '%T@ %s %p\n' 2>/dev/null | sort -n | head -n 1)

    if [ -z "$FILE_INFO" ]; then
        log_message "⚠️ 目录超标，但未发现可删除的视频文件，停止清理。"
        break
    fi

    # 解析文件信息
    FILE_SIZE_BYTES=$(echo "$FILE_INFO" | awk '{print $2}')
    FILE_PATH=$(echo "$FILE_INFO" | cut -d' ' -f3-)
    FILE_SIZE_KB=$((FILE_SIZE_BYTES / 1024))

    # 计算文件 MB 供日志显示
    FILE_MB=$(awk -v kb="$FILE_SIZE_KB" 'BEGIN {printf "%.2f", kb/1024}')

    log_message "🗑️ 正在删除最旧文件: $(basename "$FILE_PATH") (${FILE_MB} MB)"

    if rm -f "$FILE_PATH"; then
        CURRENT_SIZE_KB=$((CURRENT_SIZE_KB - FILE_SIZE_KB))
        NEW_GB=$(awk -v kb="$CURRENT_SIZE_KB" 'BEGIN {printf "%.2f", kb/1024/1024}')
        log_message "✅ 已删除。当前估算大小: ${NEW_GB} GB"
    else
        log_message "❌ 删除失败: $FILE_PATH"
        break
    fi
done

log_message "--- 自动清理任务结束 ---"