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

// JSON logging to stdout
morgan.token('json', (req, res) => JSON.stringify({
  method: req.method,
  url: req.url,
  status: res.statusCode,
  timestamp: new Date().toISOString()
}));
app.use(morgan(':json'));
app.use(express.json());

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
    console.log(JSON.stringify({ type: 'startup', message: 'Connected to Redis' }));
  } catch (err) {
    console.error(JSON.stringify({ type: 'startup', error: err.message }));
  }
})();

// Health check
app.get('/healthz', async (_, res) => {
  let dbStatus = 'ok';
  let redisStatus = 'ok';
  let code = 200;

  try {
    await db.query('SELECT 1');
  } catch {
    dbStatus = 'unreachable';
    code = 503;
  }

  try {
    await redisClient.ping();
  } catch {
    redisStatus = 'unreachable';
    code = 503;
  }

  const status = { database: dbStatus, redis: redisStatus };
  console.log(JSON.stringify({ type: 'healthz', status }));
  res.status(code).json(status);
});

// Dummy login (GET only, match Go)
app.get('/login', (_, res) => {
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
    console.error(JSON.stringify({ type: 'db', error: err.message }));
    res.status(500).send('DB error');
  }
});

// Start service
app.listen(8081, () => {
  console.log(JSON.stringify({ type: 'startup', message: 'Node.js service running on :8081' }));
});
