package mobilecore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	// 内嵌 IANA 时区库：Android 各 ROM 的 /system/usr/share/zoneinfo 不保证完整，
	// 美股交易时段判断需要 America/New_York。
	_ "time/tzdata"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
	"aipanyi/backend/models"
	"aipanyi/mobile/webui"
	"aipanyi/webserver"
)

// buildVersion 由构建脚本通过 -ldflags -X 注入，例如：
//
//	go build -ldflags "-X aipanyi/mobile/core.buildVersion=1.0.0"
var buildVersion = "0.1.0-dev"

var (
	lifecycleMu  sync.Mutex
	started      bool
	dbReady      bool
	server       *webserver.Server
	baseURL      string
	engine       *alertEngine
	engineCancel context.CancelFunc
)

// Start initializes the mobile Go core: SQLite, stock seed, local HTTP server and alert engine.
//
// It is idempotent: the Activity and the foreground service may call it concurrently;
// repeated calls return the same URL. dataDir is Android's context.filesDir.
func Start(dataDir string, n Notifier) (string, error) {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if started && server != nil {
		// 进程内重复调用：只更新 notifier（Activity 重建后通知回调对象会变）。
		setNotifier(n)
		return server.URL(), nil
	}
	if dataDir == "" {
		return "", errors.New("dataDir 不能为空")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("创建数据目录失败: %w", err)
	}
	logger.SetLogDir(filepath.Join(dataDir, "logs"))

	if !dbReady {
		db.InitWithOptions(filepath.Join(dataDir, "aipanyi.db"), db.MobileOptions())
		if err := migrateMobile(); err != nil {
			return "", err
		}
		if err := seedStockBasics(); err != nil {
			// 基础表导入失败不阻塞 App：搜索加自选不可用，但已有自选/提醒仍能工作。
			logger.SugaredLogger.Warnf("mobile: seed stock basics failed: %v", err)
		}
		dbReady = true
	}

	data.ConfigureFromSettings(data.GetSettingConfig())
	setNotifier(n)
	go marketcal.PreCache(3)

	app := newMobileApp(context.Background())
	srv, err := webserver.New(app, webui.FS, webserver.Options{
		Port:        -1, // 随机可用端口，避免与其它应用冲突
		OpenBrowser: false,
		Logger:      log.New(os.Stdout, "[aipanyi-mobile] ", log.LstdFlags),
	})
	if err != nil {
		return "", fmt.Errorf("启动本地服务失败: %w", err)
	}
	server = srv
	baseURL = srv.URL()

	ctx, cancel := context.WithCancel(context.Background())
	engineCancel = cancel
	engine = newAlertEngine()
	go engine.run(ctx)

	started = true
	logger.SugaredLogger.Infof("mobile: started at %s (version=%s)", baseURL, buildVersion)
	return baseURL, nil
}

// Stop stops the local server and alert engine. The database stays open for a later Start.
func Stop() {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if engineCancel != nil {
		engineCancel()
		engineCancel = nil
	}
	if engine != nil {
		engine.shutdown()
		engine = nil
	}
	if server != nil {
		_ = server.Shutdown()
		server = nil
	}
	started = false
	baseURL = ""
}

func migrateMobile() error {
	if db.Dao == nil {
		return errors.New("数据库未初始化")
	}
	return db.Dao.AutoMigrate(
		&data.StockBasic{},
		&data.StockInfo{},
		&data.FollowedStock{},
		&data.Settings{},
		&data.AIConfig{},
		&models.AIResponseResult{},
		&models.PromptTemplate{},
		&priceAlert{},
		&priceAlertEvent{},
		&mobileSettings{},
	)
}
