package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"spendings-backend/internal/models"
)

type TransactionsService struct {
	transactions map[string]map[string]models.Transaction // userID -> transactionID -> transaction
	mux          sync.RWMutex
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

	userTransactions, exists := ts.transactions[userID]
	if !exists {
		ts.mux.RUnlock()
		ts.mux.Lock()
		ts.transactions[userID] = getInitialTransactions()
		ts.mux.Unlock()
	}

	ts.mux.RLock()
	defer ts.mux.RUnlock()

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

		if len(categories) == 0 {
			filteredTransactions = append(filteredTransactions, transaction)
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
		ts.mux.RUnlock()
		ts.mux.Lock()
		ts.transactions[userID] = getInitialTransactions()
		ts.mux.Unlock()
	}

	ts.mux.RLock()

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

	if err := ts.validateRepeatString(req.RepeatTime); err != nil {
		return nil, fmt.Errorf("%w: invalid repeat time format: %w", models.ErrBadRequest, err)
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
		transaction.NextAppearDate = nextAppearDate
	}

	ts.mux.Lock()
	defer ts.mux.Unlock()

	// Инициализируем map для пользователя если не существует
	if ts.transactions[userID] == nil {
		ts.transactions[userID] = getInitialTransactions()
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
func (ts *TransactionsService) calculateNextAppearDate(now time.Time, repeatTime string) (time.Time, error) {
	repeatTimes := strings.Split(repeatTime, ",")
	nextAppearDate := now.AddDate(1, 0, 0)

	for _, nextEvent := range repeatTimes {
		nextEvent = strings.TrimSpace(nextEvent)
		if nextEvent == "" {
			continue
		}

		if date, err := strconv.Atoi(nextEvent); err == nil {
			var nextTime time.Time
			if date >= now.Day() {
				nextTime = time.Date(now.Year(), now.Month(), date, 0, 0, 0, 0, now.Location())
			} else {
				nextTime = time.Date(now.Year(), now.Month(), date, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
			}

			if nextAppearDate.After(nextTime) {
				nextAppearDate = nextTime
			}

			continue
		}

		if weekday, err := convertToWeekDay(nextEvent); err == nil {
			// Вычисляем следующий день недели
			daysUntilWeekday := int(weekday - now.Weekday())
			if daysUntilWeekday <= 0 {
				// День недели уже прошел на этой неделе (включая сегодня), берем следующую неделю
				daysUntilWeekday += 7
			}
			// Если daysUntilWeekday > 0, то день недели еще не прошел на этой неделе

			nextTime := now.AddDate(0, 0, daysUntilWeekday)
			nextTime = time.Date(nextTime.Year(), nextTime.Month(), nextTime.Day(), 0, 0, 0, 0, now.Location())

			if nextAppearDate.After(nextTime) {
				nextAppearDate = nextTime
			}
		}
	}

	return nextAppearDate, nil
}

func (ts *TransactionsService) validateRepeatString(repeatTime string) error {
	if repeatTime == "" {
		return nil // Пустая строка допустима
	}

	parts := strings.Split(repeatTime, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Проверяем, является ли часть числом от 1 до 31
		if num, err := strconv.Atoi(part); err == nil {
			if num < 1 || num > 31 {
				return fmt.Errorf("invalid day number: %d, must be between 1 and 31", num)
			}
			continue
		}

		// Проверяем, является ли часть днем недели
		if _, err := convertToWeekDay(part); err != nil {
			return fmt.Errorf("invalid weekday: %s, must be one of: mon, tue, wed, thu, fri, sat, sun", part)
		}
	}

	return nil
}

func convertToWeekDay(day string) (time.Weekday, error) {
	switch strings.ToLower(day) {
	case "mon", "monday":
		return time.Monday, nil
	case "tue", "tuesday":
		return time.Tuesday, nil
	case "wed", "wednesday":
		return time.Wednesday, nil
	case "thu", "thursday":
		return time.Thursday, nil
	case "fri", "friday":
		return time.Friday, nil
	case "sat", "saturday":
		return time.Saturday, nil
	case "sun", "sunday":
		return time.Sunday, nil
	default:
		return 0, fmt.Errorf("invalid weekday: %s", day)
	}
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

// ProcessAllRecurringTransactions проверяет и создает повторяющиеся транзакции для всех пользователей
func (ts *TransactionsService) ProcessAllRecurringTransactions() error {
	today := time.Now().Truncate(24 * time.Hour)

	ts.mux.Lock()
	defer ts.mux.Unlock()

	// Обрабатываем всех пользователей
	for _, userTransactions := range ts.transactions {
		// Находим транзакции, которые должны повториться сегодня
		var transactionsToProcess []models.Transaction
		for _, transaction := range userTransactions {
			if !transaction.NextAppearDate.IsZero() &&
				transaction.NextAppearDate.Truncate(24*time.Hour).Equal(today) &&
				transaction.RepeatTime != "" {
				transactionsToProcess = append(transactionsToProcess, transaction)
			}
		}

		// Создаем новые транзакции для повторения
		for _, originalTransaction := range transactionsToProcess {
			// Создаем новую транзакцию на основе оригинальной
			newTransaction := models.Transaction{
				ID:         uuid.New().String(),
				Amount:     originalTransaction.Amount,
				Title:      originalTransaction.Title,
				Category:   originalTransaction.Category,
				Date:       today,
				RepeatTime: originalTransaction.RepeatTime,
			}

			// Вычисляем следующую дату появления
			nextAppearDate, err := ts.calculateNextAppearDate(today, originalTransaction.RepeatTime)
			if err == nil {
				newTransaction.NextAppearDate = nextAppearDate
			}

			// Добавляем новую транзакцию
			userTransactions[newTransaction.ID] = newTransaction

			originalTransaction.RepeatTime = ""
			userTransactions[originalTransaction.ID] = originalTransaction
		}
	}

	return nil
}

func getInitialTransactions() map[string]models.Transaction {
	return map[string]models.Transaction{
		"c38bcbd2-e3c5-4a03-9001-bfcf763fbbdf": {
			ID: "c38bcbd2-e3c5-4a03-9001-bfcf763fbbdf",
			Amount: 100,
			Title: "Вода в зале",
			Category: "Еда",
			Date: time.Now().Add(-48 * time.Hour),
			NextAppearDate: time.Time{},
			RepeatTime: "",
		},
		"21867866-21d3-4846-bb5e-c56fbabec4f9": {
			ID: "21867866-21d3-4846-bb5e-c56fbabec4f9",
			Amount: 100,
			Title: "Кино",
			Category: "Развлечения",
			Date: time.Now().Add(-24*3 * time.Hour),
			NextAppearDate: time.Time{},
		},
		"a4075928-12c4-44e9-ac2a-0cf4230d4575": {
			ID: "a4075928-12c4-44e9-ac2a-0cf4230d4575",
			Amount: 1000,
			Title: "Зарплата",
			Category: "Доходы",
			Date: time.Now().Add(-24*7 * time.Hour),
			NextAppearDate: time.Now().AddDate(0, 1, 0),
			RepeatTime: strconv.Itoa(time.Now().Day()),
		},
	}
}