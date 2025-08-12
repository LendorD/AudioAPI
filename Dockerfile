# 1. Базовый образ с Go и Python
FROM golang:1.24-alpine AS builder

# 2. Устанавливаем зависимости
RUN apk update && apk add --no-cache \
    ca-certificates \
    git \
    openssh \
    python3 \
    py3-pip \
    python3-dev \
    build-base

# 3. Рабочая директория
WORKDIR /app

# 4. Копируем go.mod и go.sum
COPY go.mod go.sum ./

# 5. Загружаем зависимости Go
RUN go mod download

# 6. Копируем весь проект
COPY . .

# 7. Создаём виртуальное окружение Python и ставим зависимости
RUN python3 -m venv /venv \
    && . /venv/bin/activate \
    && pip install --upgrade pip \
    && pip install -r python-scripts/requirements.txt

# 8. Собираем Go бинарь
RUN go build -o main .

# 9. Финальный образ
FROM alpine:latest

WORKDIR /root/

# Копируем бинарь
COPY --from=builder /app/main .
# Копируем виртуальное окружение Python
COPY --from=builder /venv /venv
# Копируем Python-скрипты
COPY --from=builder /app/python-scripts ./python-scripts

# Указываем переменную окружения для Python
ENV PATH="/venv/bin:$PATH"

# Запуск
CMD ["./main"]
