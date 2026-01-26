const express = require('express');

const app = express();
const PORT = 8080;

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
  
  const result = number * 2;
  res.json({ 
    input: number, 
    result: result,
    operation: 'multiplica por 2'
  });
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

// 404 handler
app.use((req, res) => {
  res.status(404).json({ 
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
