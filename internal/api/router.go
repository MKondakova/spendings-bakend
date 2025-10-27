package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/cors"
	"go.uber.org/zap"

	"spendings-backend/internal/config"
	"spendings-backend/internal/models"
)

var (
	errInvalidPaginationParameter = errors.New("invalid pagination parameter")
	errEmptyID                    = errors.New("empty id")
	errEmptyName                  = errors.New("empty name")
	errJsonDecode                 = fmt.Errorf("%w: json body invalid", models.ErrBadRequest)
)

type TokenService interface {
	GenerateToken(ctx context.Context, username string, isTeacher bool) (string, error)
}

// New service interfaces for financial tracking
type StatisticsService interface {
	GetStatistics(ctx context.Context, fromDate, toDate time.Time) (*models.StatisticsResponse, error)
}

type TransactionsService interface {
	GetTransactions(ctx context.Context, categories []string, fromDate, toDate time.Time, page, pageSize int) (*models.TransactionsResponse, error)
	CreateTransaction(ctx context.Context, req models.CreateTransactionRequest) (*models.CreateTransactionResponse, error)
	DeleteTransaction(ctx context.Context, id string) error
}

type CategoriesService interface {
	GetCategories(ctx context.Context, nameFilter string) ([]models.Category, error)
	CreateCategory(ctx context.Context, category models.Category) error
}

type Router struct {
	*http.Server
	router *http.ServeMux

	tokenService        TokenService
	statisticsService   StatisticsService
	transactionsService TransactionsService
	categoriesService   CategoriesService

	logger *zap.SugaredLogger
}

func NewRouter(
	cfg config.ServerOpts,
	tokenService TokenService,
	statisticsService StatisticsService,
	transactionsService TransactionsService,
	categoriesService CategoriesService,
	authMiddleware func(next http.HandlerFunc) http.HandlerFunc,
	loggingMiddleware func(next http.HandlerFunc) http.HandlerFunc,
	logger *zap.SugaredLogger,
) *Router {
	innerRouter := http.NewServeMux()

	appRouter := &Router{
		Server: &http.Server{
			Handler:      cors.AllowAll().Handler(innerRouter),
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
			IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
		},
		router:              innerRouter,
		tokenService:        tokenService,
		statisticsService:   statisticsService,
		transactionsService: transactionsService,
		categoriesService:   categoriesService,
		logger:              logger,
	}

	innerRouter.HandleFunc("POST /createToken", authMiddleware(loggingMiddleware(appRouter.createToken)))
	innerRouter.HandleFunc("POST /createTeacherToken", authMiddleware(loggingMiddleware(appRouter.createTeacherToken)))

	// New API routes
	innerRouter.HandleFunc("GET /api/statistics", authMiddleware(loggingMiddleware(appRouter.getStatistics)))
	innerRouter.HandleFunc("GET /api/transactions", authMiddleware(loggingMiddleware(appRouter.getTransactions)))
	innerRouter.HandleFunc("POST /api/transactions", authMiddleware(loggingMiddleware(appRouter.createTransaction)))
	innerRouter.HandleFunc("DELETE /api/transactions/{id}", authMiddleware(loggingMiddleware(appRouter.deleteTransaction)))
	innerRouter.HandleFunc("GET /api/categories", authMiddleware(loggingMiddleware(appRouter.getCategories)))
	innerRouter.HandleFunc("POST /api/categories", authMiddleware(loggingMiddleware(appRouter.createCategory)))

	// Health check endpoint
	innerRouter.HandleFunc("GET /api/health", appRouter.healthCheck)

	innerRouter.HandleFunc("GET /", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "redoc-static.html")
	})

	return appRouter
}

func (r *Router) sendResponse(response http.ResponseWriter, request *http.Request, code int, buf []byte) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(code)
	_, err := response.Write(buf)
	if err != nil {
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Errorf("Error sending error response: %v", err)
	}
}

func (r *Router) sendErrorResponse(response http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, models.ErrBadRequest):
		response.WriteHeader(http.StatusBadRequest)
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Warn(err)
		r.writeError(response, request, err)

		return
	case errors.Is(err, models.ErrNotFound):
		response.WriteHeader(http.StatusNotFound)
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Warn(err)

		r.writeError(response, request, err)

		return
	case errors.Is(err, models.ErrForbidden):
		response.WriteHeader(http.StatusForbidden)
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Warn(err)

		r.writeError(response, request, err)

		return
	case errors.Is(err, models.ErrUnauthorized):
		response.WriteHeader(http.StatusUnauthorized)
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Warn(err)

		r.writeError(response, request, err)

		return
	}

	response.WriteHeader(http.StatusInternalServerError)
	r.logger.With(
		"module", "api",
		"request_url", request.Method+": "+request.URL.Path,
	).Error(err)

	r.writeError(response, request, err)
}

func (r *Router) writeError(response http.ResponseWriter, request *http.Request, err error) {
	body := map[string]string{"error": err.Error()}

	result, err := json.Marshal(body)
	if err != nil {
		r.logger.With("request_url", request.Method+": "+request.URL.Path).
			Error(fmt.Errorf("error marshalling error body: %v", err))
	}

	_, err = response.Write(result)
	if err != nil {
		r.logger.With(
			"module", "api",
			"request_url", request.Method+": "+request.URL.Path,
		).Errorf("Error sending error response: %v", err)
	}
}

func (r *Router) getStatistics(writer http.ResponseWriter, request *http.Request) {
	var fromDate, toDate time.Time
	var err error

	// Парсим параметр from, если он указан
	if fromStr := request.URL.Query().Get("from"); fromStr != "" {
		if fromDate, err = time.Parse("2006-01-02", fromStr); err != nil {
			r.sendErrorResponse(writer, request, fmt.Errorf("%w: invalid from date format: %w", models.ErrBadRequest, err))
			return
		}
	}

	// Парсим параметр to, если он указан
	if toStr := request.URL.Query().Get("to"); toStr != "" {
		if toDate, err = time.Parse("2006-01-02", toStr); err != nil {
			r.sendErrorResponse(writer, request, fmt.Errorf("%w: invalid to date format: %w", models.ErrBadRequest, err))
			return
		}
	}

	statistics, err := r.statisticsService.GetStatistics(request.Context(), fromDate, toDate)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("GetStatistics: %w", err))
		return
	}

	buf, err := json.Marshal(statistics)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusOK, buf)
}

func (r *Router) getTransactions(writer http.ResponseWriter, request *http.Request) {
	// Parse query parameters
	categories := request.URL.Query()["category"]
	if len(categories) == 1 && categories[0] == "" {
		categories = []string{}
	}

	var fromDate, toDate time.Time
	var err error

	// Парсим параметр from, если он указан
	if fromStr := request.URL.Query().Get("from"); fromStr != "" {
		if fromDate, err = time.Parse("2006-01-02", fromStr); err != nil {
			r.sendErrorResponse(writer, request, fmt.Errorf("%w: invalid from date format: %w", models.ErrBadRequest, err))
			return
		}
	}

	// Парсим параметр to, если он указан
	if toStr := request.URL.Query().Get("to"); toStr != "" {
		if toDate, err = time.Parse("2006-01-02", toStr); err != nil {
			r.sendErrorResponse(writer, request, fmt.Errorf("%w: invalid to date format: %w", models.ErrBadRequest, err))
			return
		}
	}

	page, err := getPaginationParameter(request, "page", 1)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrBadRequest, err))
		return
	}

	pageSize, err := getPaginationParameter(request, "pageSize", models.DefaultPageSize)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrBadRequest, err))
		return
	}

	transactions, err := r.transactionsService.GetTransactions(request.Context(), categories, fromDate, toDate, page, pageSize)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("GetTransactions: %w", err))
		return
	}

	buf, err := json.Marshal(transactions)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusOK, buf)
}

func (r *Router) createTransaction(writer http.ResponseWriter, request *http.Request) {
	var requestBody models.CreateTransactionRequest

	err := json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", errJsonDecode, err))
		return
	}

	response, err := r.transactionsService.CreateTransaction(request.Context(), requestBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("CreateTransaction: %w", err))
		return
	}

	buf, err := json.Marshal(response)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusCreated, buf)
}

func (r *Router) deleteTransaction(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrBadRequest, errEmptyID))
		return
	}

	err := r.transactionsService.DeleteTransaction(request.Context(), id)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("DeleteTransaction: %w", err))
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (r *Router) getCategories(writer http.ResponseWriter, request *http.Request) {
	nameFilter := request.URL.Query().Get("name")

	categories, err := r.categoriesService.GetCategories(request.Context(), nameFilter)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("GetCategories: %w", err))
		return
	}

	buf, err := json.Marshal(categories)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusOK, buf)
}

func (r *Router) createCategory(writer http.ResponseWriter, request *http.Request) {
	var requestBody models.Category

	err := json.NewDecoder(request.Body).Decode(&requestBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", errJsonDecode, err))
		return
	}

	err = r.categoriesService.CreateCategory(request.Context(), requestBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("CreateCategory: %w", err))
		return
	}

	buf, err := json.Marshal(requestBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusCreated, buf)
}

func (r *Router) createToken(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	if name == "" {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrBadRequest, errEmptyName))
		return
	}

	token, err := r.tokenService.GenerateToken(request.Context(), name, false)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("CreateToken: %w", err))
		return
	}

	responseBody := TokenResponse{
		Token: token,
	}

	buf, err := json.Marshal(responseBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusOK, buf)
}

func (r *Router) createTeacherToken(writer http.ResponseWriter, request *http.Request) {
	name := request.URL.Query().Get("name")
	if name == "" {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrBadRequest, errEmptyName))
		return
	}

	token, err := r.tokenService.GenerateToken(request.Context(), name, true)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("CreateToken: %w", err))
		return
	}

	responseBody := TokenResponse{
		Token: token,
	}

	buf, err := json.Marshal(responseBody)
	if err != nil {
		r.sendErrorResponse(writer, request, fmt.Errorf("%w: %w", models.ErrInternalServer, err))
		return
	}

	r.sendResponse(writer, request, http.StatusOK, buf)
}

func getPaginationParameter(request *http.Request, parameterName string, defaultValue int) (int, error) {
	parameter := request.URL.Query().Get(parameterName)

	if parameter == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(parameter)
	if err != nil {
		return 0, fmt.Errorf("%w %s: %w", errInvalidPaginationParameter, parameterName, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("%w %s: %d", errInvalidPaginationParameter, parameterName, value)
	}

	return value, nil
}

func (r *Router) healthCheck(writer http.ResponseWriter, _ *http.Request) {
	response := map[string]string{
		"status": "ok",
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)

	buf, _ := json.Marshal(response)
	_, _ = writer.Write(buf)
}
