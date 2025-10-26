package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"spendings-backend/internal/models"
)

var (
	errDecodePem            = errors.New("can't decode pem")
	errKeyIsNotRsaPublicKey = errors.New("key is not RSA public key")
)

type Config struct {
	ListenPort string

	PublicKey  *rsa.PublicKey  `env:"PUBLIC_KEY,notEmpty"`
	PrivateKey *rsa.PrivateKey `env:"PRIVATE_KEY,notEmpty"`

	RevokedTokens []string

	// Financial tracking data
	InitialFinancialData models.FinancialData

	ServerOpts        ServerOpts
	FeedbacksPath     string
	CreatedTokensPath string
	Host              string
}

func GetConfig(logger *zap.SugaredLogger) (*Config, error) {
	cfg := &Config{
		ListenPort: ":8080",
		ServerOpts: ServerOpts{
			ReadTimeout:          60,
			WriteTimeout:         60,
			IdleTimeout:          60,
			MaxRequestBodySizeMb: 1,
		},
		CreatedTokensPath: "data/created_tokens.csv",
		Host:              "http://eats-pages.ddns.net/uploads/",
	}

	// Загружаем заблокированные токены
	bannedTokens, err := getInitData[string]("data/blocked_tokens.json", logger)
	if err != nil {
		logger.Warnf("Can't load banned tokens from file: %v", err)
		cfg.RevokedTokens = []string{}
	} else {
		cfg.RevokedTokens = bannedTokens
	}

	// Загружаем финансовые данные
	financialData, err := getFinancialData("data/financial_data.json", logger)
	if err != nil {
		logger.Warnf("Can't load financial data from file: %v", err)
		cfg.InitialFinancialData = models.GetDefaultFinancialData()
	} else {
		cfg.InitialFinancialData = financialData
	}

	opts := env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(rsa.PublicKey{}):  ParsePubKey,
			reflect.TypeOf(rsa.PrivateKey{}): ParsePrivateKey,
		},
	}

	err = env.ParseWithOptions(cfg, opts)
	if err != nil {
		return nil, fmt.Errorf("env.ParseWithOptions: %w", err)
	}

	return cfg, nil
}

type ServerOpts struct {
	ReadTimeout          int `json:"read_timeout"`
	WriteTimeout         int `json:"write_timeout"`
	IdleTimeout          int `json:"idle_timeout"`
	MaxRequestBodySizeMb int `json:"max_request_body_size_mb"`
}

// ParsePubKey public keys loader for github.com/caarlos0/env/v11 lib.
func ParsePubKey(value string) (any, error) {
	publicKey, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("hex.DecodeString: %w", err)
	}

	pubKey, err := ParseRSAPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("keys.ParseRSAPublicKey: %w", err)
	}

	return *pubKey, nil
}

// ParsePrivateKey pkcs1 private keys loader for github.com/caarlos0/env/v11 lib.
func ParsePrivateKey(value string) (any, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("hex.DecodeString: %w", err)
	}

	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, errDecodePem
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(decoded)
	if err != nil {
		return nil, fmt.Errorf("jwt.ParseRSAPrivateKeyFromPEM: %w", err)
	}

	return *key, nil
}

func ParseRSAPublicKey(content []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, errDecodePem
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("can't parse PKIX public key: %w", err)
	}

	public, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errKeyIsNotRsaPublicKey
	}

	return public, nil
}

// loadJSONFile - обобщенная функция для загрузки JSON из файла
func loadJSONFile[T any](filePath string, logger *zap.SugaredLogger) (T, error) {
	var result T

	file, err := os.Open(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Errorf("Error while closing file %s: %v", filePath, err)
		}
	}()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return result, fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(bytes, &result); err != nil {
		return result, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

type loadable interface {
	string
}

func getInitData[T loadable](filePath string, logger *zap.SugaredLogger) ([]T, error) {
	return loadJSONFile[[]T](filePath, logger)
}

// getFinancialData загружает финансовые данные из файла
func getFinancialData(filePath string, logger *zap.SugaredLogger) (models.FinancialData, error) {
	return loadJSONFile[models.FinancialData](filePath, logger)
}
