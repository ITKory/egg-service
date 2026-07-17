# Chaos Egg API Documentation

## 📋 Обзор

Chaos Egg — это WebSocket-first сервис для мультиплеерной игры "Кликни яйцо". Сервер обрабатывает клики пользователей, синхронизирует состояние между всеми подключенными клиентами и транслирует случайные события.

ВАЖНО: Проект находится в разработке. Это не финальная версия. 

Протестировать работу можно по ссылке: [EGGCORP](http://l8s0w4sgc0kckows848w4co4.95.79.96.242.sslip.io)
---

## 🚀 Ключевые моменты проекта

### 1. Подключение к WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  console.log('Connected to Chaos Egg server');
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  handleMessage(message);
};
```

### 2. Структура сообщений

Все сообщения имеют единый формат:

```typescript
interface Message {
  type: MessageType;      // Тип сообщения
  userId?: string;        // ID пользователя (только для исходящих от сервера)
  data?: any;             // Полезная нагрузка
}
```

---

## 📤 Исходящие сообщения (Client → Server)

### Клик по яйцу (`click`)

Отправляется при каждом клике пользователя.

```typescript
{
  type: "click",
  data: {}  // Пустой объект, данные не требуются
}
```

**Пример:**
```javascript
function sendClick() {
  ws.send(JSON.stringify({
    type: "click",
    data: {}
  }));
}
```

**Важно:**
- Сервер применяет rate-limit: **максимум 1 клик каждые 50мс**
- Слишком частые клики будут проигнорированы
- Не нужно debounce/throttle на клиенте — сервер сам обработает

---

## 📥 Входящие сообщения (Server → Client)

### 1. Приветствие (`welcome`)

Приходит сразу после подключения. Содержит ваш уникальный ID и сгенерированное имя.

```typescript
{
  type: "welcome",
  data: {
    userId: "user_8f7a9b",      // Ваш уникальный ID в этой сессии
    username: "Грустный Пельмень" // Случайное отображаемое имя
  }
}
```

**Обработка:**
```javascript
function handleWelcome(data) {
  console.log(`Hello, ${data.username}! Your ID: ${data.userId}`);
  // Сохраните userId для отладки/отображения
}
```

### 2. Обновление состояния (`state_update`)

Приходит после каждого успешного клика (от любого пользователя). Содержит глобальный счетчик.

```typescript
{
  type: "state_update",
  data: {
    clickCount: 12345  // Общее количество кликов всех игроков
  }
}
```

**Обработка:**
```javascript
function handleStateUpdate(data) {
  document.getElementById('counter').textContent = data.clickCount;
}
```

### 3. Событие (`event`)

Приходит при срабатывании случайного события (каждые 100 кликов).

```typescript
{
  type: "event",
  data: {
    code: "GRAVITY_FAIL" | "INVERSION" | "BUREAUCRACY",
    message: "Внимание! Гравитация дала сбой. Ловите яйцо!",
    duration: 15000  // Длительность в миллисекундах
  }
}
```

**Типы событий:**

| Код | Название | Длительность | Описание |
|-----|----------|--------------|----------|
| `GRAVITY_FAIL` | Сбой гравитации | 15 сек | Яйцо падает вниз |
| `INVERSION` | Инверсия реальности | 10 сек | Управление инвертировано |
| `BUREAUCRACY` | Бюрократия | 20 сек | Клики требуют подтверждения |

**Обработка:**
```javascript
function handleEvent(data) {
  showNotification(data.message);
  
  // Запуск визуального эффекта
  startEventEffect(data.code, data.duration);
  
  // Автоматическое завершение через duration мс
  setTimeout(() => stopEventEffect(data.code), data.duration);
}
```

---

## 🔌 HTTP API (опционально)

### GET `/api/state` — Текущее состояние

```bash
curl http://localhost:8080/api/state
```

**Ответ:**
```json
{
  "clicks": 12345,
  "activeEvent": null  // или код активного события
}
```

### GET `/api/leaderboard` — Топ-10 игроков

```bash
curl http://localhost:8080/api/leaderboard
```

**Ответ:**
```json
{
  "leaderboard": [
    { "userId": "user_abc", "clicks": 500 },
    { "userId": "user_xyz", "clicks": 450 }
  ]
}
```
---

## ⚠️ Важные замечания

### 1. Rate Limiting
- Сервер игнорирует клики чаще **одного раза в 50мс**
- Не нужно реализовывать throttle на клиенте
- Если клик отклонён — новое состояние не придёт

### 2. Идентификация
- `userId` генерируется сервером при подключении
- Он **не сохраняется** между сессиями (нет персистентности)
- Для отображения используйте `username` (смешные имена типа "Грустный Пельмень")

### 3. События (Events)
- Срабатывают автоматически каждые **100 кликов**
- Сервер сам выбирает событие из пула
- Фронтенд должен корректно обрабатывать `duration` для таймингов эффектов

### 4. Reconnet Logic
```javascript
function connect() {
  const ws = new WebSocket('ws://localhost:8080/ws');
  
  ws.onclose = () => {
    setTimeout(connect, 3000); // Переподключение через 3 сек
  };
  
  return ws;
}
```

### 5. Ошибки
- При ошибке парсинга сообщения сервер его игнорирует
- Неподдерживаемый тип сообщения — игнорируется
- Всегда проверяйте `msg.type` перед обработкой

---

## 🧪 Тестирование

### Проверка подключения
```bash
# Установите wscat: npm install -g wscat
wscat -c ws://localhost:8080/ws
```

Ожидайте приветственное сообщение:
```json
{"type":"welcome","data":{"userId":"user_abc","username":"Имя"}}
```

### Отправка клика
```json
{"type":"click","data":{}}
```

Ожидайте обновление состояния:
```json
{"type":"state_update","data":{"clickCount":1}}
```

---

## 📦 Типы данных (TypeScript)

```typescript
type MessageType = 'click' | 'emote' | 'state_update' | 'welcome' | 'event';

interface Message<T = any> {
  type: MessageType;
  userId?: string;
  data?: T;
}

interface WelcomePayload {
  userId: string;
  username: string;
}

interface StatePayload {
  clickCount: number;
}

interface EventPayload {
  code: 'GRAVITY_FAIL' | 'INVERSION' | 'BUREAUCRACY';
  message: string;
  duration: number;
}

type IncomingMessage = 
  | Message<WelcomePayload>
  | Message<StatePayload>
  | Message<EventPayload>;

type OutgoingMessage = Message<{}>;
```

---

## 🔧 Конфигурация

| Параметр | Значение по умолчанию | Описание |
|----------|----------------------|----------|
| WebSocket URL | `ws://localhost:8080/ws` | Endpoint для подключения |
| HTTP API Base | `http://localhost:8080/api` | REST endpoints |
| Rate Limit | 50ms | Минимальный интервал между кликами |
| Event Trigger | 100 кликов | Частота срабатывания событий |


