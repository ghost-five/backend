// Настройки только из переменных окружения: так одинаково работает и на ноутбуке,
// и в контейнере, и переносится на Go без сюрпризов.
const need = (name) => {
  const v = process.env[name];
  if (!v) throw new Error('не задана переменная окружения ' + name);
  return v;
};

export const config = {
  port: Number(process.env.PORT || 8080),
  env: process.env.NODE_ENV || 'development',
  databaseUrl: process.env.DATABASE_URL || '',
  publicOrigin: process.env.PUBLIC_ORIGIN || 'http://localhost:8080',
  // Почта нужна только аккаунтам: сервер поднимается и без неё, но отправка
  // писем падает с понятной ошибкой, а не молча.
  jwtSecret: process.env.JWT_SECRET || '',
  smtpUrl: process.env.SMTP_URL || '',
  smtpFrom: process.env.SMTP_FROM || '',
  mailReady: !!process.env.SMTP_URL,
  version: process.env.GIT_SHA || 'dev',
  need
};
