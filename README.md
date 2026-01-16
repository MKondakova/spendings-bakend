# Backend для мобильного приложения

Трекер финансов - приложение для отслеживания доходов и расходов

## 📘 API

Полное описание всех методов доступно в OpenAPI [спецификации](api/openapi/spec.yaml).

### Создание JWT токенов

Для работы с API необходимо получить JWT токен. Есть два типа токенов:

#### Обычный токен (пользователь)
```bash
POST /createToken?name=username
Authorization: Bearer <existing_token>
```

**Параметры:**
- `name` (query, required) - имя пользователя для токена

**Ответ:**
```json
{
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

#### Токен преподавателя
```bash
POST /createTeacherToken?name=teacher_name
Authorization: Bearer <teacher_token>
```

**Параметры:**
- `name` (query, required) - имя преподавателя для токена

**Ответ:**
```json
{
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**⚠️ Важно:** 
- Токены преподавателя могут создавать только другие токены преподавателя
- Обычные токены могут создавать только токены преподавателя
- Все токены записываются в `data/created_tokens.csv` для аудита
- Токены можно заблокировать, добавив их ID в `data/blocked_tokens.json`

**Пример использования с curl:**
```bash
# Создать обычный токен
curl -X POST "http://localhost:8080/createToken?name=user1" \
  -H "Authorization: Bearer YOUR_TEACHER_TOKEN"

# Создать токен преподавателя (требует токен преподавателя)
curl -X POST "http://localhost:8080/createTeacherToken?name=teacher2" \
  -H "Authorization: Bearer YOUR_TEACHER_TOKEN"
```

### Финансовые операции

#### Получение статистики
```bash
GET /api/statistics?from=2025-09-01&to=2025-09-30
Authorization: Bearer <token>
```

**Параметры:**
- `from` (query, optional) - дата начала периода (YYYY-MM-DD)
- `to` (query, optional) - дата конца периода (YYYY-MM-DD)

**Ответ:**
```json
{
  "generalStatistics": {
    "income": 10000,
    "expenses": 5000,
    "balance": 5000
  },
  "balanceChangesByDate": [
    {
      "date": "2025-09-01",
      "changes": 1000
    },
    {
      "date": "2025-09-02",
      "changes": -2000
    }
  ],
  "spendingCurveInfo": [
    {
      "averageSpending": 1500,
      "currentSpending": 1000,
      "date": "2025-09-01"
    }
  ],
  "fromDate": "2025-09-01",
  "toDate": "2025-09-30"
}
```

#### Управление транзакциями

**Получение списка транзакций:**
```bash
GET /api/transactions?category=Еда&from=2025-09-01&to=2025-09-30&page=1&pageSize=10
Authorization: Bearer <token>
```

**Создание транзакции:**
```bash
POST /api/transactions
Authorization: Bearer <token>
Content-Type: application/json

{
  "amount": 1000,
  "title": "Ресторан у дома",
  "category": "Еда",
  "date": "2025-09-01",
  "repeatTime": "fri, 26, mon, 19"
}
```

**Удаление транзакции:**
```bash
DELETE /api/transactions/{id}
Authorization: Bearer <token>
```

#### Управление категориями

**Получение категорий:**
```bash
GET /api/categories?name=Еда
Authorization: Bearer <token>
```

**Создание категории:**
```bash
POST /api/categories
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Новая категория"
}
```

### Health Check

Для проверки работоспособности сервиса доступен endpoint:

```bash
GET /api/health
```

**Ответ:**
```json
{
  "status": "ok"
}
```

**Пример:**
```bash
curl http://localhost:8080/api/health
```

Этот endpoint не требует авторизации и может использоваться для мониторинга.

## 🚀 Установка и запуск

Для работы требуется установленный **nginx** и **Docker**.

1. Настроить nginx:

   ```shell
   sudo cp spendings-app.ddns.net.conf /etc/nginx/sites-available/spendings-app.ddns.net.conf
    
   sudo ln -s /etc/nginx/sites-available/spendings-app.ddns.net.conf /etc/nginx/sites-enabled/spendings-app.ddns.net.conf
    
   sudo nginx -t
   sudo nginx -s reload
   ```

2. Собрать и запустить контейнер (приложение работает на порту `8080` внутри контейнера):

   ```shell
   docker build . -t spendings-app-image
    
   docker rm -f spendings-app-app 

   docker run --env-file ./.env \
      -v "data:/root/data" \
      --restart always \
      -p 8083:8080 \
      -d --name spendings-app-app spendings-app-image:latest
   ```

   В env файле необходимо установить PUBLIC_KEY и PRIVATE_KEY. Можно сгенерировать ключи командой:
   ```shell
   openssl genrsa -out private.pem 2048
   openssl rsa -in private.pem -pubout -out public.pem
   ```

   Затем конвертировать ключи в формат base64:
   ```shell
   cat public.pem | base64 -w 0 > public.base64
   cat private.pem | base64 -w 0 > private.base64
   ```

---

## 📊 Структура данных

Приложение загружает начальные данные из JSON файлов в папке `data/`.

### Файлы данных

#### blocked_tokens.json
Содержит массив заблокированных JWT токенов.

#### created_tokens.csv
Содержит список созданных JWT токенов для отслеживания.

#### financial_data.json
Содержит финансовые данные пользователей:
```json
{
  "transactions": {
    "user_id_1": {
      "transaction_id_1": {
        "id": "1234-2222-3333-4444",
        "amount": 1000,
        "title": "Ресторан у дома",
        "category": "Еда",
        "date": "2025-09-01T00:00:00Z",
        "nextAppearDate": "2025-09-02T00:00:00Z",
        "repeatTime": "fri, 26, mon, 19"
      }
    }
  },
  "categories": {
    "user_id_1": [
      {"name": "Еда"},
      {"name": "Транспорт"},
      {"name": "Доходы"}
    ]
  }
}
```

### Автоматическое резервное копирование

Приложение автоматически создает резервные копии всех данных:

**Когда создаются бэкапы:**
- ✅ При запуске приложения
- ✅ Каждые 24 часа автоматически
- ✅ Перед завершением работы (graceful shutdown)

**Что сохраняется:**
- `transactions_backup_*.json` - транзакции пользователей
- `categories_backup_*.json` - пользовательские категории

**Структура бэкапов:**
```
data/backups/
  └── 2025-10-26/              # Дата бэкапа
      ├── transactions_backup_13-07-46.json
      └── categories_backup_13-07-46.json
```

### Восстановление из бэкапа

Для восстановления данных из бэкапа:

1. Скопировать файлы из `data/backups/YYYY-MM-DD/` в `data/`
2. Переименовать файлы бэкапа в стандартные имена:
   - `transactions_backup_*.json` → `financial_data.json`
   - `categories_backup_*.json` → `financial_data.json` (объединить с транзакциями)
3. Перезапустить приложение

**Пример:**
```bash
# Восстановление из бэкапа от 26 октября 2025
cp data/backups/2025-10-26/transactions_backup_13-07-46.json data/financial_data.json

# Перезапуск
docker restart spendings-app-app
```

### Базовые категории

Приложение автоматически предоставляет следующие базовые категории:
- Еда
- Транспорт
- Развлечения
- Здоровье
- Одежда
- Доходы
- Образование
- Подарки
- Прочее

Пользователи могут создавать дополнительные категории через API.