const express = require('express');
const os = require('os');

const app = express();
const PORT = 8080;
const HOSTNAME = os.hostname();

// Middleware
app.use(express.json());

// CORS
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  next();
});

// GET /multiplica/:number
app.get('/multiplica/:number', (req, res) => {
  const number = parseFloat(req.params.number);
  
  if (isNaN(number)) {
    return res.status(400).json({ 
      error: 'Número incorrecto',
      example: 'GET /multiplica/21'
    });
  }
  
  const envMultiplier = parseFloat(process.env.MULTIPLIER);
  const multiplier = isNaN(envMultiplier) ? 2 : envMultiplier;
  const result = number * multiplier;
  res.json({ 
    input: number, 
    multiplier: multiplier,
    result: result,
    operation: `multiplica por ${multiplier}`,
    host: HOSTNAME
  });
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

// 404 handler
app.use((req, res) => {
  res.status(404).json({ 
    applicacion: `multiplica por ${multiplier}`,
    error: 'No Encontrado',
    usage: 'GET /multiplica/:number',
    example: 'GET /multiplica/21'
  });
});

const server = app.listen(PORT, () => {
  console.log(`Server running at http://0.0.0.0:${PORT}/`);
  console.log(`Try: curl http://localhost:${PORT}/multiplica/21`);
});

process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
