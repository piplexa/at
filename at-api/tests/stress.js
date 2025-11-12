import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';

import { randomString, randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

import ws from 'k6/ws';

/**
 * Стресс тест для AT API
 *
 * Нагрузочный тест создает 4000 заданий с разным временем выполнения:
 * - 1000 заданий на текущее время плюс 10 секунд
 * - 1000 заданий на текущее время плюс 30 секунд
 * - 2000 заданий на текущее время плюс от 1 до 60 секунд (случайно)
 *
 * Пример создания задания:
 * POST: http://localhost:8080/api/v1/tasks (application/json)
 * RAW:
 * {
 *  "execute_at": "2025-11-11T15:00:00Z",
 *  "task_type": "http_callback",
 *  "payload": {
 *      "url": "http://at-api:8080/health", <- Именно такой адрес, так как вызов будет внутри контейнера at-worker
 *	    "method": "GET",
 *      "data": {
 *          "param1":"value1"
 *      }
 *  },
 *  "max_attempts": 3
 * }
 * В ответ данные о созданном задании с HTTP кодом: 201
 *
 * Запуск: k6 run stress.js
 */

import http from 'k6/http';
import exec from 'k6/execution';

// Конфигурация теста
export const options = {
  scenarios: {
    // Сценарий 1: 1000 заданий с выполнением через 10 секунд
    tasks_10sec: {
      executor: 'shared-iterations',
      vus: 10,
      iterations: 1000,
      maxDuration: '30s',
    },
    // Сценарий 2: 1000 заданий с выполнением через 30 секунд
    tasks_30sec: {
      executor: 'shared-iterations',
      vus: 10,
      iterations: 1000,
      maxDuration: '30s',
      startTime: '1s', // Небольшая задержка для разделения потоков
    },
    // Сценарий 3: 2000 заданий с случайным временем выполнения (1-60 сек)
    tasks_random: {
      executor: 'shared-iterations',
      vus: 20,
      iterations: 2000,
      maxDuration: '60s',
      startTime: '2s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% запросов должны выполняться менее 500ms
    http_req_failed: ['rate<0.01'],   // Менее 1% ошибок
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

/**
 * Создает ISO строку времени для будущего выполнения
 * @param {number} secondsFromNow - количество секунд от текущего времени
 * @returns {string} ISO строка времени
 */
function getExecuteAt(secondsFromNow) {
  const date = new Date();
  date.setSeconds(date.getSeconds() + secondsFromNow);
  return date.toISOString();
}

/**
 * Создает задание через API
 * @param {number} executeInSeconds - через сколько секунд выполнить задание
 */
function createTask(executeInSeconds) {
  const payload = {
    execute_at: getExecuteAt(executeInSeconds),
    task_type: 'http_callback',
    payload: {
      url: `http://at-api:8080/health`,
      method: 'GET',
      data: {
        test_id: randomString(8),
        iteration: __ITER,
        vu: __VU,
        timestamp: new Date().toISOString(),
      },
    },
    max_attempts: 3,
  };

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: {
      name: 'CreateTask',
      delay: `${executeInSeconds}s`,
    },
  };

  const url = `${BASE_URL}/api/v1/tasks`;

  const response = http.post(
    url,
    JSON.stringify(payload),
    params
  );

  // Проверки с выводом ошибок
  const checks = check(response, {
    'status is 201': (r) => r.status === 201,
    'has task id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.task.id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  // Если проверка не прошла, выводим подробности
//   if (!checks['status is 201'] || !checks['has task id']) {
//     console.error(`[VU ${__VU}, Iter ${__ITER}] ОШИБКА:`);
//     console.error(`  URL: ${url}`);
//     console.error(`  Status: ${response.status}`);
//     console.error(`  Body: ${response.body}`);
//     console.error(`  Error: ${response.error || 'нет'}`);
//   }

  return response;
}

/**
 * Функция setup - выполняется один раз перед началом теста
 * Используется для проверки доступности API
 */
export function setup() {
  console.log('====================================');
  console.log('Проверка доступности API...');
  console.log('BASE_URL:', BASE_URL);

  // Тестовый запрос для проверки
  const testPayload = {
    execute_at: getExecuteAt(60),
    task_type: 'http_callback',
    payload: {
      url: `http://at-api:8080/health`,
      method: 'GET',
      data: { test: 'setup' },
    },
    max_attempts: 3,
  };

  const url = `${BASE_URL}/api/v1/tasks`;
  console.log('Тестовый запрос на:', url);
  console.log('Payload:', JSON.stringify(testPayload, null, 2));

  const response = http.post(
    url,
    JSON.stringify(testPayload),
    {
      headers: { 'Content-Type': 'application/json' },
    }
  );

  console.log('Ответ сервера:');
  console.log('  Status:', response.status);
  console.log('  Body:', response.body);
  console.log('  Error:', response.error || 'нет');
  console.log('====================================\n');

  if (response.status !== 201) {
    throw new Error(`API недоступен! Status: ${response.status}, Body: ${response.body}`);
  }

  return { testTaskId: JSON.parse(response.body).id };
}

/**
 * Основная функция теста
 * Определяет, какой тип задания создавать в зависимости от сценария
 * k6 автоматически определяет текущий сценарий через exec.scenario.name
 */
export default function () {
  // k6 предоставляет имя текущего сценария через глобальный объект
  const scenarioName = exec.scenario.name;

  // Отладочный вывод для первой итерации каждого VU
//   if (__ITER === 0) {
//     console.log(`[VU ${__VU}] Запуск сценария: ${scenarioName}`);
//   }

  if (scenarioName === 'tasks_10sec') {
    // Задания с выполнением через 10 секунд
    createTask(10);
  } else if (scenarioName === 'tasks_30sec') {
    // Задания с выполнением через 30 секунд
    createTask(30);
  } else if (scenarioName === 'tasks_random') {
    // Задания со случайным временем выполнения (1-60 секунд)
    const randomDelay = randomIntBetween(1, 60);
    createTask(randomDelay);
  } else {
    console.error(`[VU ${__VU}] Неизвестный сценарий: ${scenarioName}`);
  }

  // Небольшая пауза между запросами для снижения нагрузки
  sleep(0.1);
}

/**
 * Функция teardown - выполняется после завершения теста
 * Выводит итоговую информацию
 */
export function teardown(data) {
  console.log('\n====================================');
  console.log('Тест завершен!');
//  console.log('Тестовое задание ID:', data.testTaskId);
  console.log('====================================');
}