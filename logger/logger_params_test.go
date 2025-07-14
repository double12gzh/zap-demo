package logger

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLogRotationParams 测试日志轮转参数是否生效
func TestLogRotationParams(t *testing.T) {
	// 创建临时目录用于测试
	// tempDir := t.TempDir()
	tempDir := "logs"

	tests := []struct {
		name       string
		maxSize    int
		maxBackups int
		maxAge     int
		expected   string
	}{
		{
			name:       "测试默认参数",
			maxSize:    100, // 100MB
			maxBackups: 5,
			maxAge:     30, // 30天
			expected:   "默认参数测试",
		},
		{
			name:       "测试自定义参数",
			maxSize:    10, // 10MB
			maxBackups: 3,
			maxAge:     7, // 7天
			expected:   "自定义参数测试",
		},
		{
			name:       "测试小文件参数",
			maxSize:    1, // 1MB
			maxBackups: 2,
			maxAge:     1, // 1天
			expected:   "小文件参数测试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试配置
			config := &Config{
				Level:         LogLevelInfo,
				Filename:      filepath.Join(tempDir, tt.name+".log"),
				ErrorFilename: filepath.Join(tempDir, tt.name+"_error.log"),
				MaxSize:       tt.maxSize,
				MaxBackups:    tt.maxBackups,
				MaxAge:        tt.maxAge,
				Compress:      false,
				Console:       false, // 关闭控制台输出，只测试文件
			}

			// 初始化日志器
			logger, err := NewLogger(config)
			require.NoError(t, err)
			require.NotNil(t, logger)

			// 写入足够多的日志来触发轮转
			// 每条日志大约100字节，需要写入足够多的日志来超过maxSize
			logCount := (tt.maxSize * 1024 * 1024) / 100 // 计算需要的日志条数
			if logCount < 1000 {
				logCount = 1000 // 至少写入1000条日志
			}

			for i := 0; i < logCount; i++ {
				logger.Info(tt.expected,
					zap.String("index", strconv.Itoa(i)),
					zap.String("timestamp", time.Now().Format(time.RFC3339)),
					zap.String("message", strings.Repeat("A", 50)), // 增加日志大小
				)
			}

			// 同步日志
			err = logger.Sync()
			require.NoError(t, err)

			// 检查日志文件是否存在
			assert.FileExists(t, config.Filename)

			// 检查是否生成了备份文件
			backupFiles := findBackupFiles(t, config.Filename)
			t.Logf("找到 %d 个备份文件", len(backupFiles))

			// 验证备份文件数量不超过maxBackups
			assert.LessOrEqual(t, len(backupFiles), tt.maxBackups,
				"备份文件数量应该不超过maxBackups")

			// 检查文件大小
			fileInfo, err := os.Stat(config.Filename)
			require.NoError(t, err)
			t.Logf("当前日志文件大小: %d bytes", fileInfo.Size())

			// 验证文件大小不超过maxSize
			maxSizeBytes := int64(tt.maxSize * 1024 * 1024)
			assert.LessOrEqual(t, fileInfo.Size(), maxSizeBytes,
				"当前日志文件大小应该不超过maxSize")

			// 关闭日志器
			err = logger.Close()
			require.NoError(t, err)
		})
	}
}

// TestMaxAgeParameter 专门测试maxAge参数
func TestMaxAgeParameter(t *testing.T) {
	tempDir := t.TempDir()

	// 创建配置，设置较短的maxAge用于测试
	config := &Config{
		Level:         LogLevelInfo,
		Filename:      filepath.Join(tempDir, "maxage_test.log"),
		ErrorFilename: filepath.Join(tempDir, "maxage_error_test.log"),
		MaxSize:       1, // 1MB，容易触发轮转
		MaxBackups:    5,
		MaxAge:        1, // 1天
		Compress:      true,
		Console:       false,
	}

	// 初始化日志器
	logger, err := NewLogger(config)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// 写入日志触发轮转
	for i := 0; i < 2000; i++ {
		logger.Info("测试maxAge参数",
			zap.String("index", strconv.Itoa(i)),
			zap.String("message", strings.Repeat("B", 100)),
		)
	}

	// 同步日志
	err = logger.Sync()
	require.NoError(t, err)

	// 检查备份文件
	backupFiles := findBackupFiles(t, config.Filename)
	t.Logf("maxAge测试找到 %d 个备份文件", len(backupFiles))

	// 验证备份文件存在
	assert.Greater(t, len(backupFiles), 0, "应该生成至少一个备份文件")

	// 关闭日志器
	err = logger.Close()
	require.NoError(t, err)
}

// TestMaxBackupsParameter 专门测试maxBackups参数
func TestMaxBackupsParameter(t *testing.T) {
	tempDir := t.TempDir()

	// 测试不同的maxBackups值
	testCases := []int{1, 3, 5, 10}

	for _, maxBackups := range testCases {
		t.Run("maxBackups_"+strconv.Itoa(maxBackups), func(t *testing.T) {
			config := &Config{
				Level:         LogLevelInfo,
				Filename:      filepath.Join(tempDir, "maxbackups_"+strconv.Itoa(maxBackups)+".log"),
				ErrorFilename: filepath.Join(tempDir, "maxbackups_error_"+strconv.Itoa(maxBackups)+".log"),
				MaxSize:       1, // 1MB，容易触发轮转
				MaxBackups:    maxBackups,
				MaxAge:        30,
				Compress:      true,
				Console:       false,
			}

			// 初始化日志器
			logger, err := NewLogger(config)
			require.NoError(t, err)
			require.NotNil(t, logger)

			// 写入足够多的日志来触发多次轮转
			logCount := maxBackups * 2000 // 确保触发足够多次轮转
			for i := 0; i < logCount; i++ {
				logger.Info("测试maxBackups参数",
					zap.String("index", strconv.Itoa(i)),
					zap.String("message", strings.Repeat("C", 150)),
				)
			}

			// 同步日志
			err = logger.Sync()
			require.NoError(t, err)

			// 检查备份文件数量
			backupFiles := findBackupFiles(t, config.Filename)
			t.Logf("maxBackups=%d 测试找到 %d 个备份文件", maxBackups, len(backupFiles))

			// 验证备份文件数量不超过maxBackups
			assert.LessOrEqual(t, len(backupFiles), maxBackups,
				"备份文件数量应该不超过maxBackups")

			// 关闭日志器
			err = logger.Close()
			require.NoError(t, err)
		})
	}
}

// TestMaxSizeParameter 专门测试maxSize参数
func TestMaxSizeParameter(t *testing.T) {
	tempDir := t.TempDir()

	// 测试不同的maxSize值
	testCases := []int{1, 5, 10, 50} // MB

	for _, maxSize := range testCases {
		t.Run("maxSize_"+strconv.Itoa(maxSize)+"MB", func(t *testing.T) {
			config := &Config{
				Level:         LogLevelInfo,
				Filename:      filepath.Join(tempDir, "maxsize_"+strconv.Itoa(maxSize)+".log"),
				ErrorFilename: filepath.Join(tempDir, "maxsize_error_"+strconv.Itoa(maxSize)+".log"),
				MaxSize:       maxSize,
				MaxBackups:    5,
				MaxAge:        30,
				Compress:      true,
				Console:       false,
			}

			// 初始化日志器
			logger, err := NewLogger(config)
			require.NoError(t, err)
			require.NotNil(t, logger)

			// 计算需要写入的日志条数来达到maxSize
			// 每条日志大约200字节
			logCount := (maxSize * 1024 * 1024) / 200
			if logCount < 1000 {
				logCount = 1000
			}

			for i := 0; i < logCount; i++ {
				logger.Info("测试maxSize参数",
					zap.String("index", strconv.Itoa(i)),
					zap.String("message", strings.Repeat("D", 100)),
					zap.String("timestamp", time.Now().Format(time.RFC3339Nano)),
				)
			}

			// 同步日志
			err = logger.Sync()
			require.NoError(t, err)

			// 检查当前日志文件大小
			fileInfo, err := os.Stat(config.Filename)
			require.NoError(t, err)

			maxSizeBytes := int64(maxSize * 1024 * 1024)
			t.Logf("maxSize=%dMB 测试，当前文件大小: %d bytes (%.2f MB)",
				maxSize, fileInfo.Size(), float64(fileInfo.Size())/1024/1024)

			// 验证文件大小不超过maxSize
			assert.LessOrEqual(t, fileInfo.Size(), maxSizeBytes,
				"当前日志文件大小应该不超过maxSize")

			// 检查是否生成了备份文件
			backupFiles := findBackupFiles(t, config.Filename)
			t.Logf("maxSize=%dMB 测试找到 %d 个备份文件", maxSize, len(backupFiles))

			// 关闭日志器
			err = logger.Close()
			require.NoError(t, err)
		})
	}
}

// TestCompressionParameter 测试压缩参数
func TestCompressionParameter(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		compress bool
		expected string
	}{
		{
			name:     "启用压缩",
			compress: true,
			expected: "压缩测试",
		},
		{
			name:     "禁用压缩",
			compress: false,
			expected: "非压缩测试",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Level:         LogLevelInfo,
				Filename:      filepath.Join(tempDir, tt.name+".log"),
				ErrorFilename: filepath.Join(tempDir, tt.name+"_error.log"),
				MaxSize:       1, // 1MB，容易触发轮转
				MaxBackups:    3,
				MaxAge:        30,
				Compress:      tt.compress,
				Console:       false,
			}

			// 初始化日志器
			logger, err := NewLogger(config)
			require.NoError(t, err)
			require.NotNil(t, logger)

			// 写入日志触发轮转
			for i := 0; i < 3000; i++ {
				logger.Info(tt.expected,
					zap.String("index", strconv.Itoa(i)),
					zap.String("message", strings.Repeat("E", 120)),
				)
			}

			// 同步日志
			err = logger.Sync()
			require.NoError(t, err)

			// 检查备份文件
			backupFiles := findBackupFiles(t, config.Filename)
			t.Logf("%s 找到 %d 个备份文件", tt.name, len(backupFiles))

			// 验证备份文件存在
			assert.Greater(t, len(backupFiles), 0, "应该生成至少一个备份文件")

			// 检查压缩文件（如果启用压缩）
			if tt.compress {
				compressedFiles := findCompressedFiles(t, config.Filename)
				t.Logf("%s 找到 %d 个压缩文件", tt.name, len(compressedFiles))
				assert.Greater(t, len(compressedFiles), 0, "启用压缩时应该生成压缩文件")
			}

			// 关闭日志器
			err = logger.Close()
			require.NoError(t, err)
		})
	}
}

// findBackupFiles 查找备份文件
func findBackupFiles(t *testing.T, baseFilename string) []string {
	dir := filepath.Dir(baseFilename)
	base := filepath.Base(baseFilename)
	ext := filepath.Ext(baseFilename)
	name := strings.TrimSuffix(base, ext)

	var backupFiles []string
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()
			// 匹配备份文件模式：name.ext.1, name.ext.2.gz 等
			if strings.HasPrefix(filename, name) &&
				strings.HasSuffix(filename, ext) &&
				filename != base {
				backupFiles = append(backupFiles, filepath.Join(dir, filename))
			}
		}
	}

	return backupFiles
}

// findCompressedFiles 查找压缩文件
func findCompressedFiles(t *testing.T, baseFilename string) []string {
	dir := filepath.Dir(baseFilename)
	base := filepath.Base(baseFilename)
	ext := filepath.Ext(baseFilename)
	name := strings.TrimSuffix(base, ext)

	var compressedFiles []string
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		if !entry.IsDir() {
			filename := entry.Name()
			// 匹配压缩文件模式：name.ext.1.gz, name.ext.2.gz 等
			if strings.HasPrefix(filename, name) &&
				strings.HasSuffix(filename, ".gz") {
				compressedFiles = append(compressedFiles, filepath.Join(dir, filename))
			}
		}
	}

	return compressedFiles
}
