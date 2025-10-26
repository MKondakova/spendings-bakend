package models

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const DefaultPageSize = 20

// Категория для доходов
const IncomeCategory = "Доходы"

// Auth models
type AuthTokenClaims struct {
	*jwt.RegisteredClaims
	Nickname  string `json:"nickname"`
	IsTeacher bool   `json:"isTeacher"`
}

type ContextClaimsKey struct{}

func ClaimsFromContext(ctx context.Context) *AuthTokenClaims {
	claims, _ := ctx.Value(ContextClaimsKey{}).(*AuthTokenClaims)
	return claims
}

// Transaction models
type Transaction struct {
	ID             string     `json:"id"`
	Amount         float64    `json:"amount"`
	Title          string     `json:"title"`
	Category       string     `json:"category"`
	Date           time.Time  `json:"date"`
	NextAppearDate *time.Time `json:"nextAppearDate,omitempty"`
	RepeatTime     string     `json:"repeatTime,omitempty"`
}

type CreateTransactionRequest struct {
	Amount     float64 `json:"amount"`
	Title      string  `json:"title"`
	Category   string  `json:"category"`
	Date       string  `json:"date"`
	RepeatTime string  `json:"repeatTime,omitempty"`
}

type CreateTransactionResponse struct {
	ID string `json:"id"`
}

type TransactionsResponse struct {
	CurrentPage int           `json:"currentPage"`
	TotalPages  int           `json:"totalPages"`
	Data        []Transaction `json:"data"`
}

// Statistics models
type GeneralStatistics struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Balance  float64 `json:"balance"`
}

type SpendingCurveInfo struct {
	AverageSpending float64 `json:"averageSpending"`
	CurrentSpending float64 `json:"currentSpending"`
	Date            string  `json:"date"`
}

type StatisticsResponse struct {
	GeneralStatistics    GeneralStatistics   `json:"generalStatistics"`
	BalanceChangesByDate map[string]float64  `json:"balanceChangesByDate"`
	SpendingCurveInfo    []SpendingCurveInfo `json:"spendingCurveInfo"`
	FromDate             string              `json:"fromDate"`
	ToDate               string              `json:"toDate"`
}

// Category models
type Category struct {
	Name string `json:"name"`
}

// FinancialData структура для хранения и загрузки данных финансового трекинга
type FinancialData struct {
	Transactions map[string]map[string]Transaction `json:"transactions"` // userID -> transactionID -> transaction
	Categories   map[string][]Category             `json:"categories"`   // userID -> categories
}

// GetDefaultFinancialData возвращает структуру с пустыми данными
func GetDefaultFinancialData() FinancialData {
	return FinancialData{
		Transactions: make(map[string]map[string]Transaction),
		Categories:   make(map[string][]Category),
	}
}
