const express = require('express');
const morgan = require('morgan');
const { Pool } = require('pg');

// OpenTelemetry
const { metrics } = require('@opentelemetry/api');
const { MeterProvider } = require('@opentelemetry/sdk-metrics');
const { PrometheusExporter } = require('@opentelemetry/exporter-prometheus');

// === OpenTelemetry Metrics Setup ===

const prometheusExporter = new PrometheusExporter(
  {
    port: 9464,
    startServer: true
  },
  () => {
    console.log('✅ Prometheus scrape endpoint: http://localhost:9464/metrics');
  }
);

const meterProvider = new MeterProvider();
meterProvider.addMetricReader(prometheusExporter);
metrics.setGlobalMeterProvider(meterProvider);

// Meter and Custom Metric
const meter = meterProvider.getMeter('node-service');
const loginCounter = meter.createCounter('login_requests_total', {
  description: 'Total login requests'
});

// === Express Setup ===

const app = express();

morgan.token('json', (req, res) =>
  JSON.stringify({
    method: req.method,
    url: req.url,
    status: res.statusCode,
    timestamp: new Date().toISOString()
  })
);
app.use(morgan(':json'));
app.use(express.json());

// ENV vars
const {
  DB_HOST = 'localhost',
  DB_PORT = 5432,
  DB_USER = 'postgres',
  DB_PASSWORD = 'postgres',
  DB_NAME = 'mydb'
} = process.env;

// PostgreSQL
const db = new Pool({
  host: DB_HOST,
  port: DB_PORT,
  user: DB_USER,
  password: DB_PASSWORD,
  database: DB_NAME
});

// === Endpoints ===

app.get('/login', (_, res) => {
  loginCounter.add(1);
  console.log(JSON.stringify({ type: 'login', message: 'Login endpoint hit' }));
  res.send('Logged in');
});

app.get('/healthz', async (_, res) => {
  try {
    await db.query('SELECT 1');
    res.status(200).json({ status: 'ok' });
  } catch (err) {
    console.error(err);
    res.status(503).json({ status: 'db unreachable' });
  }
});

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

// Start server
const PORT = 8081;
app.listen(PORT, () => {
  console.log(JSON.stringify({ type: 'startup', message: `Node.js service running on :${PORT}` }));
});
