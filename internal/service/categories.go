package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"spendings-backend/internal/models"
)

type CategoriesService struct {
	userCategories map[string][]models.Category // userID -> categories
	baseCategories []models.Category            // базовые категории для всех пользователей
	mux            sync.RWMutex
}

func NewCategoriesService(initialData map[string][]models.Category) *CategoriesService {
	cs := &CategoriesService{}

	if initialData != nil {
		cs.userCategories = initialData
	} else {
		cs.userCategories = make(map[string][]models.Category)
	}

	// Инициализируем базовые категории
	cs.baseCategories = []models.Category{
		{Name: "Еда"},
		{Name: "Транспорт"},
		{Name: "Развлечения"},
		{Name: "Здоровье"},
		{Name: "Одежда"},
		{Name: models.IncomeCategory},
		{Name: "Образование"},
		{Name: "Подарки"},
		{Name: "Прочее"},
	}

	return cs
}

func (cs *CategoriesService) GetCategories(ctx context.Context, nameFilter string) ([]models.Category, error) {
	userID := models.ClaimsFromContext(ctx).ID

	cs.mux.RLock()
	defer cs.mux.RUnlock()

	// Получаем категории пользователя
	userCategories, exists := cs.userCategories[userID]
	if !exists {
		userCategories = []models.Category{}
	}

	// Объединяем базовые категории с пользовательскими
	allCategories := make([]models.Category, 0, len(cs.baseCategories)+len(userCategories))

	// Добавляем базовые категории
	allCategories = append(allCategories, cs.baseCategories...)

	// Добавляем пользовательские категории
	allCategories = append(allCategories, userCategories...)

	// Применяем фильтр по названию если указан
	if nameFilter != "" {
		filteredCategories := make([]models.Category, 0)
		nameFilterLower := strings.ToLower(nameFilter)

		for _, category := range allCategories {
			if strings.HasPrefix(strings.ToLower(category.Name), nameFilterLower) {
				filteredCategories = append(filteredCategories, category)
			}
		}

		return filteredCategories, nil
	}

	return allCategories, nil
}

func (cs *CategoriesService) CreateCategory(ctx context.Context, category models.Category) error {
	userID := models.ClaimsFromContext(ctx).ID

	// Проверяем, что название категории не пустое
	if strings.TrimSpace(category.Name) == "" {
		return fmt.Errorf("%w: category name cannot be empty", models.ErrBadRequest)
	}

	cs.mux.Lock()
	defer cs.mux.Unlock()

	// Инициализируем список категорий для пользователя если не существует
	if cs.userCategories[userID] == nil {
		cs.userCategories[userID] = make([]models.Category, 0)
	}

	// Проверяем, что категория с таким названием еще не существует
	for _, existingCategory := range cs.userCategories[userID] {
		if strings.EqualFold(existingCategory.Name, category.Name) {
			return fmt.Errorf("%w: category with name '%s' already exists", models.ErrBadRequest, category.Name)
		}
	}

	// Проверяем, что категория не конфликтует с базовыми
	for _, baseCategory := range cs.baseCategories {
		if strings.EqualFold(baseCategory.Name, category.Name) {
			return fmt.Errorf("%w: category with name '%s' already exists in base categories", models.ErrBadRequest, category.Name)
		}
	}

	// Добавляем новую категорию
	cs.userCategories[userID] = append(cs.userCategories[userID], category)

	return nil
}

// GetBackupData возвращает данные для бэкапа
func (cs *CategoriesService) GetBackupData() interface{} {
	cs.mux.RLock()
	defer cs.mux.RUnlock()

	// Создаем копию данных для бэкапа
	backupData := make(map[string][]models.Category)
	for userID, categories := range cs.userCategories {
		backupCategories := make([]models.Category, len(categories))
		for i, category := range categories {
			backupCategories[i] = models.Category{
				Name: category.Name,
			}
		}
		backupData[userID] = backupCategories
	}

	return backupData
}

// GetBackupFileName возвращает имя файла для бэкапа
func (cs *CategoriesService) GetBackupFileName() string {
	return "categories"
}
