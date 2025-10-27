package service

import (
	"context"
	"fmt"
	"time"

	"spendings-backend/internal/models"
)

type TransactionsProvider interface {
	GetAllTransactions(ctx context.Context, fromDate, toDate time.Time) ([]models.Transaction, error)
}


type StatisticsService struct {
	transactionsService TransactionsProvider
}

func NewStatisticsService(transactionsService TransactionsProvider) *StatisticsService {
	return &StatisticsService{
		transactionsService: transactionsService,
	}
}

func (ss *StatisticsService) GetStatistics(ctx context.Context, fromDate, toDate time.Time) (*models.StatisticsResponse, error) {
	// Если даты не указаны, используем текущий месяц
	if fromDate.IsZero() && toDate.IsZero() {
		now := time.Now()
		fromDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		toDate = fromDate.AddDate(0, 1, -1) // последний день месяца
	}

	// Получаем все транзакции пользователя за период
	transactions, err := ss.transactionsService.GetAllTransactions(ctx, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// Вычисляем общую статистику
	generalStats := ss.calculateGeneralStatistics(transactions)

	// Вычисляем изменения баланса по датам
	balanceChanges := ss.calculateBalanceChangesByDate(transactions, fromDate, toDate)

	// Вычисляем информацию о кривой трат
	spendingCurve, err := ss.calculateSpendingCurve(ctx, transactions, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate spending curve: %w", err)
	}

	return &models.StatisticsResponse{
		GeneralStatistics:    generalStats,
		BalanceChangesByDate: balanceChanges,
		SpendingCurveInfo:    spendingCurve,
		FromDate:             fromDate.Format("2006-01-02"),
		ToDate:               toDate.Format("2006-01-02"),
	}, nil
}

// calculateGeneralStatistics вычисляет общую статистику
func (ss *StatisticsService) calculateGeneralStatistics(transactions []models.Transaction) models.GeneralStatistics {
	var income, expenses float64

	for _, transaction := range transactions {
		if transaction.Category == models.IncomeCategory {
			income += transaction.Amount
		} else {
			expenses += transaction.Amount
		}
	}

	balance := income - expenses

	return models.GeneralStatistics{
		Income:   income,
		Expenses: expenses,
		Balance:  balance,
	}
}

// calculateBalanceChangesByDate вычисляет изменения баланса по датам
func (ss *StatisticsService) calculateBalanceChangesByDate(transactions []models.Transaction, fromDate, toDate time.Time) map[string]float64 {
	balanceChanges := make(map[string]float64)

	// Инициализируем все даты в периоде нулевыми значениями
	currentDate := fromDate
	for !currentDate.After(toDate) {
		dateStr := currentDate.Format("2006-01-02")
		balanceChanges[dateStr] = 0
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	// Добавляем изменения от транзакций
	for _, transaction := range transactions {
		dateStr := transaction.Date.Format("2006-01-02")
		if transaction.Category == models.IncomeCategory {
			balanceChanges[dateStr] += transaction.Amount
		} else {
			balanceChanges[dateStr] -= transaction.Amount
		}
	}

	return balanceChanges
}

// calculateSpendingCurve вычисляет информацию о кривой трат
func (ss *StatisticsService) calculateSpendingCurve(ctx context.Context, transactions []models.Transaction, fromDate, toDate time.Time) ([]models.SpendingCurveInfo, error) {
	// Получаем все траты пользователя для вычисления средних значений
	allExpenses, err := ss.transactionsService.GetAllTransactions(ctx, time.Time{}, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all transactions: %w", err)
	}

	// Фильтруем только траты (не доходы)
	var allExpenseTransactions []models.Transaction
	for _, transaction := range allExpenses {
		if transaction.Category != models.IncomeCategory {
			allExpenseTransactions = append(allExpenseTransactions, transaction)
		}
	}

	// Группируем траты по датам
	expensesByDate := make(map[string]float64)
	for _, transaction := range allExpenseTransactions {
		dateStr := transaction.Date.Format("2006-01-02")
		expensesByDate[dateStr] += transaction.Amount
	}

	// Группируем транзакции текущего периода по датам
	transactionsByDate := make(map[string][]models.Transaction)
	for _, transaction := range transactions {
		dateStr := transaction.Date.Format("2006-01-02")
		transactionsByDate[dateStr] = append(transactionsByDate[dateStr], transaction)
	}

	// Создаем список всех дат в периоде
	var dates []string
	currentDate := fromDate
	for !currentDate.After(toDate) {
		dates = append(dates, currentDate.Format("2006-01-02"))
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	// Вычисляем кривую трат для каждой даты
	var spendingCurve []models.SpendingCurveInfo

	for _, dateStr := range dates {
		// Текущие траты за этот день
		currentSpending := 0.0
		if dayTransactions, exists := transactionsByDate[dateStr]; exists {
			for _, transaction := range dayTransactions {
				if transaction.Category != models.IncomeCategory {
					currentSpending += transaction.Amount
				}
			}
		}

		// Средние траты - среднее значение трат в этот день в другие месяцы
		// Для простоты используем среднее значение всех трат за этот день
		averageSpending := 0.0
		if expensesByDate[dateStr] > 0 {
			averageSpending = expensesByDate[dateStr]
		}

		spendingCurve = append(spendingCurve, models.SpendingCurveInfo{
			AverageSpending: averageSpending,
			CurrentSpending: currentSpending,
			Date:            dateStr,
		})
	}

	return spendingCurve, nil
}
