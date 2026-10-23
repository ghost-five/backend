// Каркас сервера. Пока умеет только сказать, что жив: этого достаточно, чтобы
// собрать образ, выложить его на ВМ и убедиться, что весь конвейер работает.
// Дальше сюда приезжают маршруты из docs/api/openapi.yaml.
import { createServer } from 'node:http';
import { config } from './config.js';

const started = Date.now();

const routes = {
  // Пути без префикса — для healthcheck внутри контейнера, с префиксом — наружу.
  '/healthz': health,
  '/api/v1/healthz': health
};

function health(req, res) {
  send(res, 200, {
    status: 'ok',
    version: config.version,
    uptime: Math.round((Date.now() - started) / 1000)
  });
}

function send(res, code, body) {
  const text = JSON.stringify(body);
  res.writeHead(code, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(text),
    'cache-control': 'no-store'
  });
  res.end(text);
}

const server = createServer((req, res) => {
  const path = (req.url || '/').split('?')[0];
  const handler = routes[path];
  if (!handler) {
    send(res, 404, { error: { code: 'not_found', message: 'Такого пути нет' } });
    return;
  }
  try {
    handler(req, res);
  } catch (e) {
    console.error('ошибка в обработчике', path, e);
    send(res, 500, { error: { code: 'internal', message: 'Внутренняя ошибка' } });
  }
});

server.listen(config.port, () => {
  console.log('ghost-five api: порт ' + config.port + ', сборка ' + config.version);
});

// Контейнер останавливают сигналом: закрываем соединения сами, иначе docker
// добивает процесс через десять секунд и обрывает чужие запросы.
for (const sig of ['SIGTERM', 'SIGINT']) {
  process.on(sig, () => {
    console.log('останавливаюсь по ' + sig);
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 5000).unref();
  });
}
