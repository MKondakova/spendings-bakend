package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"spendings-backend/internal/models"
)

type TransactionsService struct {
	transactions map[string]map[string]models.Transaction // userID -> transactionID -> transaction
	mux          sync.RWMutex
}

// TransactionsServiceInterface интерфейс для TransactionsService
type TransactionsServiceInterface interface {
	GetTransactions(ctx context.Context, categories []string, fromDate, toDate time.Time, page, pageSize int) (*models.TransactionsResponse, error)
	GetAllTransactions(ctx context.Context, fromDate, toDate time.Time) ([]models.Transaction, error)
	CreateTransaction(ctx context.Context, req models.CreateTransactionRequest) (*models.CreateTransactionResponse, error)
	DeleteTransaction(ctx context.Context, id string) error
}

func NewTransactionsService(initialData map[string]map[string]models.Transaction) *TransactionsService {
	ts := &TransactionsService{}

	if initialData != nil {
		ts.transactions = initialData
	} else {
		ts.transactions = make(map[string]map[string]models.Transaction)
	}

	return ts
}

func (ts *TransactionsService) GetTransactions(ctx context.Context, categories []string, fromDate, toDate time.Time, page, pageSize int) (*models.TransactionsResponse, error) {
	userID := models.ClaimsFromContext(ctx).ID

	ts.mux.RLock()
	defer ts.mux.RUnlock()

	userTransactions, exists := ts.transactions[userID]
	if !exists {
		return &models.TransactionsResponse{
			CurrentPage: page,
			TotalPages:  0,
			Data:        []models.Transaction{},
		}, nil
	}

	// Конвертируем map в slice и применяем фильтры
	var filteredTransactions []models.Transaction
	for _, transaction := range userTransactions {
		// Фильтр по датам
		if !fromDate.IsZero() && transaction.Date.Before(fromDate) {
			continue
		}
		if !toDate.IsZero() && transaction.Date.After(toDate) {
			continue
		}

		// Фильтр по категориям
		if len(categories) > 0 {
			found := false
			for _, category := range categories {
				if transaction.Category == category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filteredTransactions = append(filteredTransactions, transaction)
	}

	// Сортируем по дате (новые сначала)
	sort.Slice(filteredTransactions, func(i, j int) bool {
		return filteredTransactions[i].Date.After(filteredTransactions[j].Date)
	})

	// Применяем пагинацию
	transactionsAmount := len(filteredTransactions)
	totalPages := int(math.Ceil(float64(transactionsAmount) / float64(pageSize)))

	paginationStart := (page - 1) * pageSize

	if paginationStart >= transactionsAmount {
		return &models.TransactionsResponse{
			CurrentPage: page,
			TotalPages:  totalPages,
			Data:        []models.Transaction{},
		}, nil
	}

	paginationEnd := paginationStart + pageSize
	if paginationEnd > transactionsAmount {
		paginationEnd = transactionsAmount
	}

	paginatedTransactions := filteredTransactions[paginationStart:paginationEnd]

	return &models.TransactionsResponse{
		CurrentPage: page,
		TotalPages:  totalPages,
		Data:        paginatedTransactions,
	}, nil
}

func (ts *TransactionsService) GetAllTransactions(ctx context.Context, fromDate, toDate time.Time) ([]models.Transaction, error) {
	userID := models.ClaimsFromContext(ctx).ID

	ts.mux.RLock()
	defer ts.mux.RUnlock()

	userTransactions, exists := ts.transactions[userID]
	if !exists {
		return []models.Transaction{}, nil
	}

	// Конвертируем map в slice и применяем фильтры по датам
	var filteredTransactions []models.Transaction
	for _, transaction := range userTransactions {
		// Фильтр по датам
		if !fromDate.IsZero() && transaction.Date.Before(fromDate) {
			continue
		}
		if !toDate.IsZero() && transaction.Date.After(toDate) {
			continue
		}

		filteredTransactions = append(filteredTransactions, transaction)
	}

	// Сортируем по дате (новые сначала)
	sort.Slice(filteredTransactions, func(i, j int) bool {
		return filteredTransactions[i].Date.After(filteredTransactions[j].Date)
	})

	return filteredTransactions, nil
}

func (ts *TransactionsService) CreateTransaction(ctx context.Context, req models.CreateTransactionRequest) (*models.CreateTransactionResponse, error) {
	userID := models.ClaimsFromContext(ctx).ID

	// Парсим дату
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid date format: %w", models.ErrBadRequest, err)
	}

	// Генерируем ID транзакции
	transactionID := uuid.New().String()

	// Создаем транзакцию
	transaction := models.Transaction{
		ID:         transactionID,
		Amount:     req.Amount,
		Title:      req.Title,
		Category:   req.Category,
		Date:       date,
		RepeatTime: req.RepeatTime,
	}

	// Обрабатываем повторяющиеся транзакции
	if req.RepeatTime != "" {
		nextAppearDate, err := ts.calculateNextAppearDate(date, req.RepeatTime)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid repeat time format: %w", models.ErrBadRequest, err)
		}
		transaction.NextAppearDate = &nextAppearDate
	}

	ts.mux.Lock()
	defer ts.mux.Unlock()

	// Инициализируем map для пользователя если не существует
	if ts.transactions[userID] == nil {
		ts.transactions[userID] = make(map[string]models.Transaction)
	}

	// Сохраняем транзакцию
	ts.transactions[userID][transactionID] = transaction

	return &models.CreateTransactionResponse{
		ID: transactionID,
	}, nil
}

func (ts *TransactionsService) DeleteTransaction(ctx context.Context, id string) error {
	userID := models.ClaimsFromContext(ctx).ID

	ts.mux.Lock()
	defer ts.mux.Unlock()

	userTransactions, exists := ts.transactions[userID]
	if !exists {
		return nil
	}

	delete(userTransactions, id)
	return nil
}

// calculateNextAppearDate вычисляет следующую дату появления для повторяющихся транзакций
func (ts *TransactionsService) calculateNextAppearDate(currentDate time.Time, repeatTime string) (time.Time, error) {
	// Простая реализация для примера
	// В реальном приложении здесь была бы более сложная логика
	// для обработки различных форматов повторения

	// Для простоты добавляем 7 дней
	nextDate := currentDate.AddDate(0, 0, 7)
	return nextDate, nil
}

// GetBackupData возвращает данные для бэкапа
func (ts *TransactionsService) GetBackupData() interface{} {
	ts.mux.RLock()
	defer ts.mux.RUnlock()

	// Создаем копию данных для бэкапа
	backupData := make(map[string]map[string]models.Transaction)
	for userID, transactions := range ts.transactions {
		backupTransactions := make(map[string]models.Transaction)
		for transactionID, transaction := range transactions {
			backupTransaction := models.Transaction{
				ID:             transaction.ID,
				Amount:         transaction.Amount,
				Title:          transaction.Title,
				Category:       transaction.Category,
				Date:           transaction.Date,
				NextAppearDate: transaction.NextAppearDate,
			}
			backupTransactions[transactionID] = backupTransaction
		}
		backupData[userID] = backupTransactions
	}

	return backupData
}

// GetBackupFileName возвращает имя файла для бэкапа
func (ts *TransactionsService) GetBackupFileName() string {
	return "transactions"
}
