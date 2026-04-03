#!/bin/bash
# Flutter APK 打包脚本
# 用法: ./build-apk.sh

set -e

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
FLUTTER_DIR="$PROJECT_DIR"
ANDROID_DIR="$FLUTTER_DIR/android"

echo "=== Flutter APK 打包开始 ==="
echo "项目目录: $FLUTTER_DIR"
echo ""

# 配置国内镜像（阿里云）
configure_mirrors() {
    echo "配置国内镜像（阿里云）..."

    # 配置 settings.gradle.kts
    SETTINGS_FILE="$ANDROID_DIR/settings.gradle.kts"
    if [ -f "$SETTINGS_FILE" ]; then
        if ! grep -q "maven.aliyun.com" "$SETTINGS_FILE"; then
            # 备份原文件
            cp "$SETTINGS_FILE" "$SETTINGS_FILE.bak"

            # 在 repositories 块中添加阿里云镜像
            sed -i 's/repositories {/repositories {\n        maven { url = uri("https:\/\/maven.aliyun.com\/repository\/public") }\n        maven { url = uri("https:\/\/maven.aliyun.com\/repository\/google") }\n        maven { url = uri("https:\/\/maven.aliyun.com\/repository\/gradle-plugin") }/' "$SETTINGS_FILE"
            echo "  已更新 settings.gradle.kts"
        else
            echo "  settings.gradle.kts 已有阿里云镜像配置"
        fi
    fi

    # 配置 build.gradle.kts
    BUILD_FILE="$ANDROID_DIR/build.gradle.kts"
    if [ -f "$BUILD_FILE" ]; then
        if ! grep -q "maven.aliyun.com" "$BUILD_FILE"; then
            # 备份原文件
            cp "$BUILD_FILE" "$BUILD_FILE.bak"

            # 在 allprojects repositories 块中添加阿里云镜像
            sed -i 's/repositories {/repositories {\n        maven { url = uri("https:\/\/maven.aliyun.com\/repository\/public") }\n        maven { url = uri("https:\/\/maven.aliyun.com\/repository\/google") }/' "$BUILD_FILE"
            echo "  已更新 build.gradle.kts"
        else
            echo "  build.gradle.kts 已有阿里云镜像配置"
        fi
    fi

    echo "  国内镜像配置完成"
}

# 停止 Gradle 守护进程
echo "停止 Gradle 守护进程..."
cd "$ANDROID_DIR" && ./gradlew --stop 2>/dev/null || true

# 清理 Gradle transforms 缓存（解决缓存损坏问题）
echo "清理 Gradle 缓存..."
rm -rf "$HOME/.gradle/caches/8.14/transforms" 2>/dev/null || true
rm -rf "$HOME/.gradle/caches/8.14/kotlin-script" 2>/dev/null || true
rm -rf "$HOME/.gradle/caches/8.14/fileHashes" 2>/dev/null || true
rm -rf "$HOME/.gradle/caches/8.14/jars-9" 2>/dev/null || true

# 配置国内镜像
configure_mirrors

# 清理 Flutter 构建
echo "清理 Flutter 构建..."
cd "$FLUTTER_DIR" && flutter clean

# 获取依赖
echo "获取依赖..."
cd "$FLUTTER_DIR" && flutter pub get

# 构建 Release APK
echo "构建 Release APK..."
cd "$FLUTTER_DIR" && flutter build apk --release

echo ""
echo "=== APK 打包完成 ==="
echo "输出路径: $FLUTTER_DIR/build/app/outputs/flutter-apk/app-release.apk"
