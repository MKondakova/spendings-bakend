# Backend для мобильного приложения

Трекер финансов

## 📘 API

Полное описание всех методов доступно в OpenAPI [спецификации](api/openapi/spec.yaml).

### Создание JWT токенов

Для работы с API необходимо получить JWT токен. Есть два типа токенов:

#### Обычный токен (студент)
```bash
POST /api/createToken?name=username
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
POST /api/createTeacherToken?name=teacher_name
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
curl -X POST "http://localhost:8080/api/createToken?name=student1" \
  -H "Authorization: Bearer YOUR_TEACHER_TOKEN"

# Создать токен преподавателя (требует токен преподавателя)
curl -X POST "http://localhost:8080/api/createTeacherToken?name=teacher2" \
  -H "Authorization: Bearer YOUR_TEACHER_TOKEN"
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
   sudo cp spendings-pages.ddns.net.conf /etc/nginx/sites-available/spendings-pages.ddns.net.conf
    
   sudo ln -s /etc/nginx/sites-available/spendings-pages.ddns.net.conf /etc/nginx/sites-enabled/spendings-pages.ddns.net.conf
    
   sudo nginx -t
   sudo nginx -s reload
   ```

2. Собрать и запустить контейнер (приложение работает на порту `8080` внутри контейнера):

   ```shell
   docker build . -t spendings-pages-image
    
   docker rm -f spendings-pages-app 

   docker run --env-file ./.env \
      -v "data:/root/data" \
      --restart always \
      -p 8081:8080 \
      -d --name spendings-pages-app spendings-pages-image:latest
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

### Автоматическое резервное копирование

Приложение автоматически создает резервные копии всех данных:

**Когда создаются бэкапы:**
- ✅ При запуске приложения
- ✅ Каждые 24 часа автоматически
- ✅ Перед завершением работы (graceful shutdown)

**Что сохраняется:**
- `user_profiles.json` - профили пользователей
- `cart_items.json` - корзины
- `user_favourites.json` - избранное
- `orders.json` - заказы
- `wallet_data.json` - данные кошельков

**Структура бэкапов:**
```
data/backups/
  └── 2025-10-21/              # Дата бэкапа
      ├── user_profiles_backup_14-30-00.json
      ├── cart_items_backup_14-30-00.json
      ├── user_favourites_backup_14-30-00.json
      ├── orders_backup_14-30-00.json
      └── wallet_data_backup_14-30-00.json
```

### Восстановление из бэкапа

Для восстановления данных из бэкапа:

1. Скопировать файлы из `data/backups/YYYY-MM-DD/` в `data/`
2. Переименовать файлы бэкапа в стандартные имена:
   - `user_profiles_backup_*.json` → `user_profiles.json`
   - `cart_items_backup_*.json` → `cart_items.json`
   - `user_favourites_backup_*.json` → `user_favourites.json`
   - `orders_backup_*.json` → `orders.json`
   - `wallet_data_backup_*.json` → `wallet_data.json`
3. Перезапустить приложение

**Пример:**
```bash
# Восстановление из бэкапа от 21 октября 2025
cp data/backups/2025-10-21/user_profiles_backup_14-30-00.json data/user_profiles.json
cp data/backups/2025-10-21/cart_items_backup_14-30-00.json data/cart_items.json
cp data/backups/2025-10-21/user_favourites_backup_14-30-00.json data/user_favourites.json
cp data/backups/2025-10-21/orders_backup_14-30-00.json data/orders.json
cp data/backups/2025-10-21/wallet_data_backup_14-30-00.json data/wallet_data.json

# Перезапуск
docker restart spendings-pages-app
```

