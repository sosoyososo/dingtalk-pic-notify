#!/bin/bash

UPLOADER="./oss-uploader"

# 检查依赖
check_dependencies() {
    LIBCAMERA_BIN=$(command -v libcamera-still)
    if [ -z "$LIBCAMERA_BIN" ] || [ ! -x "$LIBCAMERA_BIN" ]; then
        echo "错误: libcamera-still 未安装或无法执行"
        exit 1
    fi
    
    if [ ! -f "$UPLOADER" ]; then
        echo "错误: 上传工具 $UPLOADER 不存在"
        exit 1
    fi
}

# 拍照并上传
capture_and_upload() {
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    IMAGE_FILE="capture_$TIMESTAMP.jpg"
    
    # 拍照 (使用libcamera-still)
    libcamera-still -o "$IMAGE_FILE" --quality 100
    
    # 上传到OSS并发送钉钉通知
    if "$UPLOADER" "$IMAGE_FILE"; then
        # 上传成功后删除本地图片
        rm -f "$IMAGE_FILE"
        echo "图片已上传并删除本地文件: $IMAGE_FILE"
    else
        echo "上传失败，保留本地文件: $IMAGE_FILE"
    fi
}

# 主流程
main() {
    check_dependencies
    capture_and_upload
}

main
