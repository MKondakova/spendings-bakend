package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"spendings-backend/internal/api"
	"spendings-backend/internal/config"
	"spendings-backend/internal/service"
	"spendings-backend/pkg/runner"
)

type Application struct {
	cfg *config.Config

	tokenService        *service.TokenService
	statisticsService   *service.StatisticsService
	transactionsService *service.TransactionsService
	categoriesService   *service.CategoriesService
	backupService       *service.BackupService
	logger              *zap.SugaredLogger

	errChan chan error
	wg      sync.WaitGroup
	ready   bool
}

func New() *Application {
	return &Application{
		errChan: make(chan error),
	}
}

func (a *Application) Start(ctx context.Context) error {
	if err := a.initConfigAndLogger(); err != nil {
		return err
	}

	if err := a.initServices(); err != nil {
		return err
	}

	if err := a.initRouter(ctx); err != nil {
		return err
	}

	// Запускаем сервис бэкапа в отдельной горутине
	a.wg.Go(func() {
		a.backupService.Start(ctx)
	})

	// Приложение готово к работе
	a.ready = true

	return nil
}

func (a *Application) Ready() bool {
	return a.ready
}

func (a *Application) HandleGracefulShutdown(ctx context.Context, cancel context.CancelFunc) error {
	var appErr error

	errWg := sync.WaitGroup{}

	errWg.Go(func() {
		for err := range a.errChan {
			cancel()
			a.logger.Error(err)
			appErr = err
		}
	})

	<-ctx.Done()

	a.logger.Info("Shutdown initiated, waiting for services to stop...")
	a.wg.Wait()

	// Выполняем финальный бекап перед завершением работы
	a.logger.Info("Creating final backup before shutdown...")
	if err := a.backupService.PerformBackup(); err != nil {
		a.logger.Errorf("Failed to create final backup: %v", err)
	} else {
		a.logger.Info("Final backup completed successfully")
	}

	close(a.errChan)
	errWg.Wait()

	a.logger.Info("Graceful shutdown completed")
	return appErr
}

func (a *Application) initConfigAndLogger() error {
	if err := a.initLogger(); err != nil {
		return fmt.Errorf("can't init logger: %w", err)
	}

	if err := a.initConfig(); err != nil {
		return fmt.Errorf("can't init config: %w", err)
	}

	return nil
}

func (a *Application) initConfig() error {
	var err error

	a.cfg, err = config.GetConfig(a.logger)
	if err != nil {
		return fmt.Errorf("can't parse config: %w", err)
	}

	return nil
}

func (a *Application) initLogger() error {
	zapLog, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("can't create logger: %w", err)
	}

	a.logger = zapLog.Sugar()

	return nil
}

func (a *Application) initServices() error {
	// Инициализируем сервисы с данными из конфига
	a.tokenService = service.NewTokenService(a.cfg.PrivateKey, a.cfg.CreatedTokensPath)
	a.transactionsService = service.NewTransactionsService(a.cfg.InitialFinancialData.Transactions)
	a.categoriesService = service.NewCategoriesService(a.cfg.InitialFinancialData.Categories)
	a.statisticsService = service.NewStatisticsService(a.transactionsService)

	// Инициализируем сервис бэкапа (каждые 24 часа)
	a.backupService = service.NewBackupService(a.logger, "data", 24*time.Hour)

	// Регистрируем все сервисы для бэкапа
	a.backupService.RegisterBackupable(a.transactionsService)
	a.backupService.RegisterBackupable(a.categoriesService)

	return nil
}

func (a *Application) initRouter(ctx context.Context) error {
	authMiddleware := api.NewAuthMiddleware(a.cfg.PublicKey, a.logger, a.cfg.RevokedTokens).JWTAuth
	loggingMiddleware := api.NewLoggerMiddleware(a.logger).Middleware

	router := api.NewRouter(
		a.cfg.ServerOpts,
		a.tokenService,
		a.statisticsService,
		a.transactionsService,
		a.categoriesService,
		authMiddleware,
		loggingMiddleware,
		a.logger,
	)

	if err := runner.RunServer(ctx, router, a.cfg.ListenPort, a.errChan, &a.wg); err != nil {
		return fmt.Errorf("can't run public router: %w", err)
	}

	return nil
}
