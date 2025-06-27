const request = require('supertest');
const express = require('express');

const app = express();
app.get('/healthz', (_, res) => res.send('ok'));
app.post('/login', (_, res) => res.send('Logged in'));
app.get('/products', (_, res) => res.json(["Product A", "Product B"]));

describe('Node Service', () => {
  it('should return 200 for /healthz', async () => {
    const res = await request(app).get('/healthz');
    expect(res.statusCode).toBe(200);
    expect(res.text).toBe('ok');
  });

  it('should return "Logged in"', async () => {
    const res = await request(app).post('/login');
    expect(res.text).toBe('Logged in');
  });

  it('should return product list', async () => {
    const res = await request(app).get('/products');
    expect(res.body).toEqual(["Product A", "Product B"]);
  });
});
