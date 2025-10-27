package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RecurringTransactionsService сервис для обработки повторяющихся транзакций
type RecurringTransactionsService struct {
	transactionsService *TransactionsService
	logger              *zap.SugaredLogger
	stopChan            chan struct{}
}

// NewRecurringTransactionsService создает новый сервис для повторяющихся транзакций
func NewRecurringTransactionsService(transactionsService *TransactionsService, logger *zap.SugaredLogger) *RecurringTransactionsService {
	return &RecurringTransactionsService{
		transactionsService: transactionsService,
		logger:              logger,
		stopChan:            make(chan struct{}),
	}
}

// Start запускает периодическую проверку повторяющихся транзакций
func (rts *RecurringTransactionsService) Start(ctx context.Context) {
	rts.logger.Info("Starting recurring transactions service")

	// Проверяем каждые 24 часа в полночь
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Выполняем первую проверку сразу при запуске
	if err := rts.processRecurringTransactions(); err != nil {
		rts.logger.Errorf("Failed to process recurring transactions on startup: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := rts.processRecurringTransactions(); err != nil {
				rts.logger.Errorf("Failed to process recurring transactions: %v", err)
			}
		case <-rts.stopChan:
			rts.logger.Info("Recurring transactions service stopped")
			return
		case <-ctx.Done():
			rts.logger.Info("Recurring transactions service stopped by context")
			return
		}
	}
}

// Stop останавливает сервис
func (rts *RecurringTransactionsService) Stop() {
	close(rts.stopChan)
}

// processRecurringTransactions обрабатывает повторяющиеся транзакции
func (rts *RecurringTransactionsService) processRecurringTransactions() error {
	rts.logger.Info("Processing recurring transactions")

	if err := rts.transactionsService.ProcessAllRecurringTransactions(); err != nil {
		return err
	}

	rts.logger.Info("Recurring transactions processed successfully")
	return nil
}
