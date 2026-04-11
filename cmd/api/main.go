package main

import (
	"context"
	"fmt"
	"llm-agent-platform/internal/knowledge/domain"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	applogger "llm-agent-platform/internal/shared/logger"

	"llm-agent-platform/internal/config"
	"llm-agent-platform/internal/knowledge/application/usecase/ingestion"
	"llm-agent-platform/internal/knowledge/application/usecase/retrieval"
	"llm-agent-platform/internal/knowledge/chunking"
	"llm-agent-platform/internal/knowledge/infrastructure/embedding"
	"llm-agent-platform/internal/knowledge/repository"
	"llm-agent-platform/pkg/milvus"
)

type application struct {
	cfg          *config.Config
	logger       *zap.Logger
	db           *gorm.DB
	milvusClient client.Client
	esClient     *elasticsearch.Client
	httpServer   *http.Server
	ingestionUC  *ingestion.IngestionUsecase
	retrievalUC  *retrieval.HybridRetriever
}

func main() {
	app, err := newApplication()
	if err != nil {
		log.Fatalf("failed to create application: %v", err)
	}

	if err := app.run(); err != nil {
		app.logger.Fatal("failed to run application", zap.Error(err))
	}
}

func newApplication() (*application, error) {
	// 1. 加载配置
	cfg := config.Load()

	// 2. 初始化日志
	logger, err := newLogger(cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("日志初始化失败: %w", err)
	}
	logger.Info("配置加载完成", zap.String("port", cfg.Server.Port))

	// 3. 初始化基础设施
	db, err := newDatabase(cfg.Database, logger)
	if err != nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}
	logger.Info("数据库连接成功")

	milvusClient, err := newMilvusClient(cfg.Milvus, logger)
	if err != nil {
		return nil, fmt.Errorf("Milvus初始化失败: %w", err)
	}
	logger.Info("Milvus连接成功")

	esClient, err := newElasticsearchClient(cfg.ES, logger)
	if err != nil {
		return nil, fmt.Errorf("ES初始化失败: %w", err)
	}
	logger.Info("ES连接成功")

	// 4. 初始化Collection（如果不存在）
	if err := initMilvusCollection(milvusClient, cfg.Milvus, logger); err != nil {
		return nil, fmt.Errorf("Milvus Collection初始化失败: %w", err)
	}

	// 5. 组装UseCase
	ingestionUC, retrievalUC, err := newUseCases(cfg, db, milvusClient, esClient, logger)
	if err != nil {
		return nil, fmt.Errorf("UseCase组装失败: %w", err)
	}
	logger.Info("UseCase组装完成")

	// 6. 创建HTTP服务器
	httpServer := newHTTPServer(cfg, logger, ingestionUC, retrievalUC)

	return &application{
		cfg:          cfg,
		logger:       logger,
		db:           db,
		milvusClient: milvusClient,
		esClient:     esClient,
		ingestionUC:  ingestionUC,
		retrievalUC:  retrievalUC,
		httpServer:   httpServer,
	}, nil
}

func (app *application) run() error {
	// 启动 HTTP 服务
	go func() {
		app.logger.Info("HTTP服务启动",
			zap.String("addr", app.httpServer.Addr),
			zap.Duration("read_timeout", app.cfg.Server.ReadTimeout),
		)
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatal("HTTP服务启动失败", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	sig := <-quit

	app.logger.Info("收到关闭信号", zap.String("signal", sig.String()))

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), app.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := app.shutdown(ctx); err != nil {
		app.logger.Error("应用关闭出错", zap.Error(err))
		return err
	}

	app.logger.Info("应用已安全退出")
	return nil
}

func (app *application) shutdown(ctx context.Context) error {
	// 1. 关闭HTTP服务器
	if err := app.httpServer.Shutdown(ctx); err != nil {
		app.logger.Error("HTTP服务器关闭失败", zap.Error(err))
	}

	// 2. 关闭数据库连接
	if sqlDB, err := app.db.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			app.logger.Error("数据库连接关闭失败", zap.Error(err))
		}
	}

	// 3. 关闭Milvus连接
	if app.milvusClient != nil {
		app.milvusClient.Close()
	}

	// 4. 刷新日志
	_ = app.logger.Sync()

	return nil
}

func newLogger(cfg config.LogConfig) (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var core zapcore.Core
	if cfg.Output == "file" && cfg.FilePath != "" {
		// 文件输出
		writer, _, err := zap.Open(cfg.FilePath)
		if err != nil {
			return nil, err
		}
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			writer,
			level,
		)
	} else {
		// 标准输出
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
	}

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}

func newDatabase(cfg config.DatabaseConfig, zapLogger *zap.Logger) (*gorm.DB, error) {
	gormLogger := applogger.NewGormZapLogger(zapLogger,
		applogger.WithSlowThreshold(time.Second),
		applogger.WithGormLogLevel(gormlogger.Warn),
	)

	db, err := gorm.Open(postgres.Open(buildDSN(cfg)), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 连接池配置
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("数据库连接验证失败: %w", err)
	}

	zapLogger.Info("数据库连接成功",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("db", cfg.DBName),
	)

	return db, nil
}

func buildDSN(cfg config.DatabaseConfig) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
}

func newMilvusClient(cfg config.MilvusConfig, logger *zap.Logger) (client.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	c, err := client.NewClient(ctx, client.Config{
		Address:  fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName:   cfg.DBName,
		Username: cfg.Username,
		Password: cfg.Password,
	})
	if err != nil {
		return nil, err
	}

	// 验证连接
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		return nil, fmt.Errorf("Milvus连接验证失败: %w", err)
	}

	return c, nil
}

func initMilvusCollection(client client.Client, cfg config.MilvusConfig, logger *zap.Logger) error {
	ctx := context.Background()

	// 检查Collection是否存在
	exists, err := client.HasCollection(ctx, cfg.CollectionName)
	if err != nil {
		return err
	}

	if exists {
		logger.Info("Milvus Collection已存在", zap.String("name", cfg.CollectionName))
		return nil
	}

	// 创建Collection
	manager := milvus.NewCollectionManager(client)
	if err := manager.CreateCollection(ctx, cfg.CollectionName, cfg.VectorDim); err != nil {
		return err
	}

	logger.Info("Milvus Collection创建成功",
		zap.String("name", cfg.CollectionName),
		zap.Int("dim", cfg.VectorDim),
	)
	return nil
}

func newElasticsearchClient(cfg config.ESConfig, logger *zap.Logger) (*elasticsearch.Client, error) {
	esCfg := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		APIKey:    cfg.APIKey,
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, err
	}

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	res, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("ES连接验证失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES连接返回错误: %s", res.String())
	}

	return client, nil
}

func newUseCases(
	cfg *config.Config,
	db *gorm.DB,
	milvusClient client.Client,
	esClient *elasticsearch.Client,
	logger *zap.Logger,
) (*ingestion.IngestionUsecase, *retrieval.HybridRetriever, error) {
	// Embedding客户端
	embedClient := embedding.NewEmbeddingClient()

	// Rerank客户端
	rerankClient := embedding.NewRerankClient()

	// 分块器
	chunker := chunking.NewRecursiveCharacterChunker(chunking.ChunkConfig{
		ChunkSize:   cfg.Workflow.MaxWorkers * 10, // 根据并发调整
		OverlapSize: 50,
		Separators:  []string{"\n\n", "\n", "。", "！", "？", ".", " ", ""},
	})

	// Repository层
	metaRepo := repository.NewPGMetadataRepo(db)
	vectorRepo := repository.NewMilvusRepo(milvusClient, cfg.Milvus.CollectionName)
	keywordRepo := repository.NewESRepo(esClient, cfg.ES.IndexName)

	// Reranker
	reranker := retrieval.NewCrossEncoderReranker(rerankClient)

	// UseCase
	ingestionUC := ingestion.NewIngestionUsecase(
		embedClient,
		vectorRepo,
		keywordRepo,
		metaRepo,
		chunker,
		logger,
	)

	retrievalUC := retrieval.NewHybridRetriever(
		vectorRepo,
		keywordRepo,
		metaRepo,
		reranker,
		float64(cfg.Agent.MaxIterations), // 复用配置或单独定义权重
		0.3,                              // beta: 关键词权重
		logger,
	)

	return ingestionUC, retrievalUC, nil
}

func newHTTPServer(
	cfg *config.Config,
	logger *zap.Logger,
	ingestionUC *ingestion.IngestionUsecase,
	retrievalUC *retrieval.HybridRetriever,
) *http.Server {
	// 设置Gin模式
	if cfg.Log.Level == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := setupRouter(cfg, logger, ingestionUC, retrievalUC, metaRepo)

	return &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}
}

func requestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		logger.Info("HTTP请求",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func errorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			logger.Error("请求处理错误",
				zap.String("path", c.Request.URL.Path),
				zap.Error(err),
			)
			c.JSON(-1, gin.H{
				"error":   "internal_server_error",
				"message": err.Error(),
			})
		}
	}
}

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func readyHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 实际应该检查数据库、Milvus、ES连接状态
		c.JSON(200, gin.H{
			"status":    "ready",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func handleIngest(uc *ingestion.IngestionUsecase, metaRepo repository.MetadataRepo, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title   string                 `json:"title" binding:"required"`
			Content string                 `json:"content" binding:"required"`
			Meta    map[string]interface{} `json:"meta"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}

		ctx := c.Request.Context()
		docID := uuid.New().String()

		doc := &domain.Document{
			ID:        docID,
			Title:     req.Title,
			Content:   req.Content,
			Metadata:  req.Meta,
			Status:    "PROCESSING",
			CreatedAt: time.Now(),
		}

		if err := metaRepo.CreateDocument(ctx, doc); err != nil {
			logger.Error("Failed to create document", zap.Error(err), zap.String("doc_id", docID))
			c.JSON(500, gin.H{"error": "failed_to_create_document", "message": err.Error()})
			return
		}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("panic recovered", zap.String("doc_id", docID), zap.Any("panic", r))
					// 尝试防止文档永远处于PROCESSING状态无法重试
					_ = metaRepo.UpdateDocumentStatus(context.Background(), docID, "FAILED")
				}
			}()

			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			_ = metaRepo.UpdateDocumentStatus(bgCtx, docID, "PROCESSING")

			err := uc.Ingest(bgCtx, doc)
			if err != nil {
				_ = metaRepo.UpdateDocumentStatus(bgCtx, docID, "FAILED")
				logger.Error("Ingestion failed", zap.Error(err))
				return
			}

			_ = metaRepo.UpdateDocumentStatus(bgCtx, docID, "COMPLETED")

		}()

		c.JSON(http.StatusAccepted, gin.H{
			"status": "PENDING",
			"doc_id": docID,
		})
	}
}

func handleRetrieve(uc *retrieval.HybridRetriever, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Query string `json:"query" binding:"required"`
			TopK  int    `json:"top_k"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid_request", "message": err.Error()})
			return
		}

		if req.TopK == 0 {
			req.TopK = 10
		}

		// TODO: 调用uc.Retrieve()，需要先获取queryVector
		logger.Info("检索请求", zap.String("query", req.Query), zap.Int("top_k", req.TopK))

		c.JSON(200, gin.H{
			"query":  req.Query,
			"chunks": []interface{}{}, // 实际结果
		})
	}
}

func setupRouter(
	cfg *config.Config,
	logger *zap.Logger,
	ingestionUC *ingestion.IngestionUsecase,
	retrievalUC *retrieval.HybridRetriever,
	metaRepo repository.MetadataRepo,
) *gin.Engine {
	r := gin.New()
	r.Use(requestLogger(logger), corsMiddleware(), errorHandler(logger))

	// 健康检查
	r.GET("/health", healthHandler)
	r.GET("/ready", readyHandler(cfg))

	// 业务路由
	api := r.Group("/api/v1")
	{
		api.POST("/documents", handleIngest(ingestionUC, metaRepo, logger))
		api.POST("/retrieve", handleRetrieve(retrievalUC, logger))
	}

	return r
}
