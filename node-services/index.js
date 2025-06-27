const express = require('express');
const morgan = require('morgan');
const { Pool } = require('pg');
const redis = require('redis');

// OpenTelemetry Tracing
const { NodeTracerProvider } = require('@opentelemetry/sdk-trace-node');
const { SimpleSpanProcessor, ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-base');

// OpenTelemetry Metrics
const { MeterProvider } = require('@opentelemetry/sdk-metrics');
const { PrometheusExporter } = require('@opentelemetry/exporter-prometheus');

// Setup tracing
const tracerProvider = new NodeTracerProvider();
tracerProvider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
tracerProvider.register();

// Setup metrics
const prometheusExporter = new PrometheusExporter({ port: 9464, endpoint: '/metrics' }, () => {
  console.log('Prometheus scrape endpoint ready at http://localhost:9464/metrics');
});
const meterProvider = new MeterProvider();
meterProvider.addMetricReader(prometheusExporter);

// Create custom metric
const meter = meterProvider.getMeter('node-service');
const loginCounter = meter.createCounter('login_requests_total', {
  description: 'Total login requests'
});

const app = express();

// JSON logging
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

// PostgreSQL
const db = new Pool({
  host: DB_HOST,
  port: DB_PORT,
  user: DB_USER,
  password: DB_PASSWORD,
  database: DB_NAME
});

// Redis
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

// Dummy login
app.get('/login', (_, res) => {
  loginCounter.add(1); // Increment metric
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
