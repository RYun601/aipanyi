package mobilecore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
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
		scheduleMonitorRefresh()
		return server.URL(), nil
	}
	if dataDir == "" {
		return "", errors.New("dataDir 不能为空")
	}
	// Android 的 Go 进程默认可能是 UTC；不设置的话 time.Now() 会让 A 股交易时段判断整体偏移 8 小时。
	// 本应用面向 A/HK/US 行情，统一把进程本地时区设为北京时间（美股判断内部会转纽约时间）。
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = loc
	}
	rootDir, stats, err := prepareDataRoot(dataDir)
	if err != nil {
		return "", err
	}
	logger.SetLogDir(filepath.Join(rootDir, "logs"))
	logger.SugaredLogger.Infof("mobile: core start dataDir=%s root=%s", dataDir, rootDir)
	if stats.moved > 0 || stats.skipped > 0 || len(stats.errors) > 0 {
		logger.SugaredLogger.Infof(
			"mobile: migrate legacy data moved=%d skipped=%d errors=%d",
			stats.moved, stats.skipped, len(stats.errors),
		)
		for _, msg := range stats.errors {
			logger.SugaredLogger.Warnf("mobile: migrate legacy data: %s", msg)
		}
	}
	data.SetDataDir(rootDir)

	if !dbReady {
		db.InitWithOptions(filepath.Join(rootDir, "aipanyi.db"), db.MobileOptions())
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
	scheduleMonitorRefresh()
	logger.SugaredLogger.Infof("mobile: started at %s (version=%s)", baseURL, buildVersion)
	return baseURL, nil
}

// Stop stops the local server and alert engine. The database stays open for a later Start.
func Stop() {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	logger.SugaredLogger.Info("mobile: core stop")
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
