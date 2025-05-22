package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/go-yaml/yaml"
)

// Config 配置文件结构体
type Config struct {
	AliyunOSS struct {
		Endpoint        string `yaml:"endpoint"`
		AccessKeyID     string `yaml:"access_key_id"`
		AccessKeySecret string `yaml:"access_key_secret"`
		BucketName      string `yaml:"bucket_name"`
	} `yaml:"aliyun_oss"`
	DingtalkBot struct {
		Webhook string `yaml:"webhook"`
		Secret  string `yaml:"secret"`
	} `yaml:"dingtalk_bot"`
}

// 计算钉钉机器人签名
func calculateSign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// uploadToOSS 上传文件到阿里云OSS
func uploadToOSS(config Config, filePath string) (string, error) {
	client, err := oss.New(config.AliyunOSS.Endpoint, config.AliyunOSS.AccessKeyID, config.AliyunOSS.AccessKeySecret)
	if err != nil {
		return "", fmt.Errorf("创建OSS客户端失败: %v", err)
	}

	bucket, err := client.Bucket(config.AliyunOSS.BucketName)
	if err != nil {
		return "", fmt.Errorf("获取存储空间失败: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	now := time.Now()
	datePath := now.Format("20060102")
	millis := now.UnixNano() / 1e6 % 1000
	baseName := filepath.Base(filePath)
	objectName := fmt.Sprintf("raspi/%s/%d_%s", datePath, millis, baseName)
	err = bucket.PutObject(objectName, file)
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %v", err)
	}

	url := fmt.Sprintf("https://%s.%s/%s", config.AliyunOSS.BucketName, config.AliyunOSS.Endpoint, objectName)
	return url, nil
}

// sendToDingtalk 发送消息到钉钉机器人(带签名)
func sendToDingtalk(config Config, message string) error {
	timestamp := time.Now().UnixNano() / 1e6
	sign := calculateSign(config.DingtalkBot.Secret, timestamp)

	webhookURL := fmt.Sprintf("%s&timestamp=%d&sign=%s", 
		config.DingtalkBot.Webhook, 
		timestamp, 
		sign)

	currentTime := time.Now().Format("2006年01月02日 15:04:05")
	markdownContent := fmt.Sprintf("### 树莓派图片上传通知\n**时间**: %s\n\n![图片预览](%s)\n\n[点击查看原图](%s)", currentTime, message, message)
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": "文件上传通知",
			"text": markdownContent,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(msgBytes))
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应失败: %v", err)
		}
		return fmt.Errorf("钉钉API返回错误: %s, %s", resp.Status, string(body))
	}

	return nil
}

// loadConfig 加载配置文件
func loadConfig(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("读取配置文件失败: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("解析YAML失败: %v", err)
	}

	return config, nil
}

func listAndCleanOldFolders(config Config) error {
	client, err := oss.New(config.AliyunOSS.Endpoint, config.AliyunOSS.AccessKeyID, config.AliyunOSS.AccessKeySecret)
	if err != nil {
		return fmt.Errorf("创建OSS客户端失败: %v", err)
	}

	bucket, err := client.Bucket(config.AliyunOSS.BucketName)
	if err != nil {
		return fmt.Errorf("获取存储空间失败: %v", err)
	}

	// 列出raspi/下的所有文件夹
	prefix := "raspi/"
	marker := ""
	for {
		lor, err := bucket.ListObjects(oss.Prefix(prefix), oss.Delimiter("/"), oss.Marker(marker))
		if err != nil {
			return fmt.Errorf("列出文件夹失败: %v", err)
		}

		// 处理每个日期文件夹
		for _, prefix := range lor.CommonPrefixes {
			// 提取日期部分(raspi/YYYYMMDD/)
			dateStr := strings.TrimPrefix(prefix, "raspi/")
			dateStr = strings.TrimSuffix(dateStr, "/")
			
			// 解析日期
			folderDate, err := time.Parse("20060102", dateStr)
			if err != nil {
				continue // 跳过格式不正确的文件夹
			}

			// 检查是否超过3天
			if time.Since(folderDate) > 3*24*time.Hour {
				// 获取文件夹下所有对象
				objects, err := bucket.ListObjects(oss.Prefix(prefix))
				if err != nil {
					return fmt.Errorf("列出文件夹%s内容失败: %v", prefix, err)
				}
				
				// 准备要删除的对象列表
				var keys []string
				for _, obj := range objects.Objects {
					keys = append(keys, obj.Key)
				}
				
				// 删除所有对象
				_, err = bucket.DeleteObjects(keys)
				if err != nil {
					return fmt.Errorf("删除文件夹%s失败: %v", prefix, err)
				}
				fmt.Printf("已删除过期文件夹: %s\n", prefix)
			}
		}

		if !lor.IsTruncated {
			break
		}
		marker = lor.NextMarker
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("用法: ./main <文件路径>")
		os.Exit(1)
	}
	filePath := os.Args[1]

	// 先清理过期文件夹
	config, err := loadConfig("config.yaml")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	err = listAndCleanOldFolders(config)
	if err != nil {
		fmt.Printf("清理过期文件夹失败: %v\n", err)
		// 不退出，继续上传
	}

	fileURL, err := uploadToOSS(config, filePath)
	if err != nil {
		fmt.Printf("上传文件失败: %v\n", err)
		os.Exit(1)
	}

	message := fileURL
	err = sendToDingtalk(config, message)
	if err != nil {
		fmt.Printf("发送钉钉消息失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("文件上传并通知成功!")
}
