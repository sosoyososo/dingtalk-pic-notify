#!/bin/bash

VERSION="1.0.1"
DIST_DIR="dist"
BIN_DIR="${DIST_DIR}/bin"
RELEASE_DIR="${DIST_DIR}/release"

# 清理并创建目录
rm -rf ${DIST_DIR}
mkdir -p ${BIN_DIR} ${RELEASE_DIR}

# 编译函数
build_for() {
    local os=$1
    local arch=$2
    local suffix=$3
    
    echo "Building for ${os} ${arch}..."
    output="${BIN_DIR}/oss-uploader-${VERSION}-${os}-${arch}${suffix}"
    GOOS=${os} GOARCH=${arch} go build -o ${output}
    
    # 打包
    tar -czf "${RELEASE_DIR}/oss-uploader-${VERSION}-${os}-${arch}.tar.gz" -C ${BIN_DIR} "oss-uploader-${VERSION}-${os}-${arch}${suffix}"
    
    # 生成校验文件
    shasum -a 256 "${RELEASE_DIR}/oss-uploader-${VERSION}-${os}-${arch}.tar.gz" > "${RELEASE_DIR}/oss-uploader-${VERSION}-${os}-${arch}.tar.gz.sha256"
}

# 编译各平台
build_for darwin arm64 ""
build_for darwin amd64 "" 
build_for linux arm64 ""
build_for linux amd64 ""
# 树莓派3B (armv7)
GOARM=7 build_for linux arm ""

# Windows版本如果需要可以添加
# build_for windows amd64 ".exe"

echo "Build complete. Release files:"
ls -lh ${RELEASE_DIR}/
