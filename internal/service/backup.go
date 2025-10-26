package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Backupable интерфейс для объектов, которые нужно бэкапить
type Backupable interface {
	GetBackupData() interface{}
	GetBackupFileName() string
}

// BackupService сервис для автоматического бэкапа данных
type BackupService struct {
	logger      *zap.SugaredLogger
	backupables []Backupable
	dataDir     string
	interval    time.Duration
	stopChan    chan struct{}
	mu          sync.RWMutex
}

// NewBackupService создает новый сервис бэкапа
func NewBackupService(logger *zap.SugaredLogger, dataDir string, interval time.Duration) *BackupService {
	return &BackupService{
		logger:      logger,
		backupables: make([]Backupable, 0),
		dataDir:     dataDir,
		interval:    interval,
		stopChan:    make(chan struct{}),
	}
}

// RegisterBackupable регистрирует объект для бэкапа
func (bs *BackupService) RegisterBackupable(backupable Backupable) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.backupables = append(bs.backupables, backupable)
	bs.logger.Infof("Registered backupable: %s", backupable.GetBackupFileName())
}

// Start запускает периодический бэкап
func (bs *BackupService) Start(ctx context.Context) {
	bs.logger.Info("Starting backup service")

	// Выполняем первый бэкап сразу при запуске
	if err := bs.PerformBackup(); err != nil {
		bs.logger.Errorf("Initial backup failed: %v", err)
	}

	ticker := time.NewTicker(bs.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := bs.PerformBackup(); err != nil {
				bs.logger.Errorf("Backup failed: %v", err)
			}
		case <-bs.stopChan:
			bs.logger.Info("Backup service stopped")
			return
		case <-ctx.Done():
			bs.logger.Info("Backup service stopped by context")
			return
		}
	}
}

// Stop останавливает сервис бэкапа
func (bs *BackupService) Stop() {
	close(bs.stopChan)
}

// PerformBackup выполняет бэкап всех зарегистрированных объектов
func (bs *BackupService) PerformBackup() error {
	bs.mu.RLock()
	backupables := make([]Backupable, len(bs.backupables))
	copy(backupables, bs.backupables)
	bs.mu.RUnlock()

	if len(backupables) == 0 {
		bs.logger.Debug("No backupables registered, skipping backup")
		return nil
	}

	bs.logger.Info("Starting backup process")

	// Создаем директорию для бэкапов если она не существует
	backupDir := filepath.Join(bs.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Создаем поддиректорию с текущей датой
	timestamp := time.Now().Format("2006-01-02")
	dateDir := filepath.Join(backupDir, timestamp)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return fmt.Errorf("failed to create date directory: %w", err)
	}

	successCount := 0
	for _, backupable := range backupables {
		if err := bs.backupObject(backupable, dateDir); err != nil {
			bs.logger.Errorf("Failed to backup %s: %v", backupable.GetBackupFileName(), err)
		} else {
			successCount++
		}
	}

	bs.logger.Infof("Backup completed: %d/%d objects backed up successfully", successCount, len(backupables))
	return nil
}

// backupObject создает бэкап отдельного объекта
func (bs *BackupService) backupObject(backupable Backupable, backupDir string) error {
	fileName := backupable.GetBackupFileName()
	if fileName == "" {
		return fmt.Errorf("empty backup file name")
	}

	data := backupable.GetBackupData()
	if data == nil {
		return fmt.Errorf("no backup data available")
	}

	// Добавляем timestamp к имени файла
	timestamp := time.Now().Format("15-04-05")
	backupFileName := fmt.Sprintf("%s_backup_%s.json", fileName, timestamp)
	filePath := filepath.Join(backupDir, backupFileName)

	// Сериализуем данные в JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup data: %w", err)
	}

	// Записываем в файл
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	bs.logger.Debugf("Successfully backed up %s to %s", fileName, filePath)
	return nil
}
