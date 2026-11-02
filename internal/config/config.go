// Package config читает настройки сервера. Только из переменных окружения, без
// файлов: так одинаково работает и на ноутбуке, и в контейнере.
package config

import (
	"fmt"
	"strconv"
)

type Config struct {
	Port         int
	Env          string // development или production
	DatabaseURL  string
	PublicOrigin string
	JWTSecret    string
	// Почта нужна только аккаунтам: сервер поднимается и без неё, но отправка
	// писем падает с понятной ошибкой, а не молча.
	SMTPURL  string
	SMTPFrom string
	Version  string // хеш коммита, из которого собран образ
}

// Load собирает настройки через getenv — в боевом коде это os.Getenv, в тестах
// подставляется своя функция. Обязательные переменные проверяет тот, кому они
// нужны: каркасу без базы и почты требовать их рано.
func Load(getenv func(string) string) (Config, error) {
	or := func(name, def string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return def
	}

	port, err := strconv.Atoi(or("PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT должен быть номером порта, а не %q", getenv("PORT"))
	}

	return Config{
		Port:         port,
		Env:          or("APP_ENV", "development"),
		DatabaseURL:  getenv("DATABASE_URL"),
		PublicOrigin: or("PUBLIC_ORIGIN", "http://localhost:8080"),
		JWTSecret:    getenv("JWT_SECRET"),
		SMTPURL:      getenv("SMTP_URL"),
		SMTPFrom:     getenv("SMTP_FROM"),
		Version:      or("GIT_SHA", "dev"),
	}, nil
}

func (c Config) MailReady() bool { return c.SMTPURL != "" }
