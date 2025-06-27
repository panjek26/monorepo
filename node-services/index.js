const express = require('express');
const morgan = require('morgan');
const { Pool } = require('pg');
const redis = require('redis');

// OpenTelemetry setup
const { NodeTracerProvider } = require('@opentelemetry/sdk-trace-node');
const { ConsoleSpanExporter, SimpleSpanProcessor } = require('@opentelemetry/sdk-trace-base');

const provider = new NodeTracerProvider();
provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
provider.register();

const app = express();
app.use(express.json());

// JSON logger for HTTP requests
app.use(morgan((tokens, req, res) => {
  return JSON.stringify({
    time: tokens.date(req, res, 'iso'),
    method: tokens.method(req, res),
    url: tokens.url(req, res),
    status: parseInt(tokens.status(req, res), 10),
    content_length: tokens.res(req, res, 'content-length'),
    response_time_ms: parseFloat(tokens['response-time'](req, res)),
    user_agent: tokens['user-agent'](req, res)
  });
}));

// ENV vars
const {
  DB_HOST = 'localhost',
  DB_PORT = 5432,
  DB_USER = 'postgres',
  DB_PASSWORD = 'postgres',
  DB_NAME = 'mydb',
  REDIS_HOST = 'localhost',
  REDIS_PORT = 6379
} = process.env;

// PostgreSQL setup
const db = new Pool({
  host: DB_HOST,
  port: DB_PORT,
  user: DB_USER,
  password: DB_PASSWORD,
  database: DB_NAME
});

// Redis setup
const redisClient = redis.createClient({
  url: `redis://${REDIS_HOST}:${REDIS_PORT}`
});

redisClient.on('error', err => console.error('Redis error:', err));

(async () => {
  try {
    await redisClient.connect();
    console.log('Connected to Redis');
  } catch (err) {
    console.error('Redis connect error:', err);
  }
})();

// Health check
app.get('/healthz', async (_, res) => {
  let dbStatus = 'ok';
  let redisStatus = 'ok';
  let code = 200;

  try {
    await db.query('SELECT 1');
  } catch (err) {
    dbStatus = 'unreachable';
    code = 503;
  }

  try {
    await redisClient.ping();
  } catch (err) {
    redisStatus = 'unreachable';
    code = 503;
  }

  const status = { database: dbStatus, redis: redisStatus };
  console.log(JSON.stringify({ type: 'healthcheck', status }));
  res.status(code).json(status);
});

// Dummy login
app.post('/login', (_, res) => {
  console.log(JSON.stringify({ type: 'login', message: 'Login endpoint hit' }));
  res.send('Logged in');
});

// Product list from DB
app.get('/products', async (_, res) => {
  try {
    const result = await db.query('SELECT name FROM products');
    const names = result.rows.map(r => r.name);
    res.json(names);
  } catch (err) {
    console.error(JSON.stringify({ type: 'db_error', error: err.message }));
    res.status(500).send('DB error');
  }
});

const PORT = 8081;
app.listen(PORT, () => {
  console.log(`Node.js service running on :${PORT}`);
});
