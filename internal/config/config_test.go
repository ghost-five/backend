package config

import "testing"

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 || c.Env != "development" || c.Version != "dev" {
		t.Errorf("умолчания: %+v", c)
	}
	if c.MailReady() {
		t.Error("без SMTP_URL почта не готова")
	}
}

func TestFromEnv(t *testing.T) {
	c, err := Load(env(map[string]string{
		"PORT": "9000", "APP_ENV": "production", "GIT_SHA": "abc123", "SMTP_URL": "smtps://x@y:465",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 9000 || c.Env != "production" || c.Version != "abc123" || !c.MailReady() {
		t.Errorf("из окружения: %+v", c)
	}
}

func TestBadPort(t *testing.T) {
	for _, p := range []string{"восемь", "0", "70000"} {
		if _, err := Load(env(map[string]string{"PORT": p})); err == nil {
			t.Errorf("PORT=%q должен быть ошибкой", p)
		}
	}
}
