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

// Función para verificar si un número es primo
function esPrimo(num) {
  if (num <= 1) return false;
  if (num <= 3) return true;
  if (num % 2 === 0 || num % 3 === 0) return false;
  
  for (let i = 5; i * i <= num; i += 6) {
    if (num % i === 0 || num % (i + 2) === 0) return false;
  }
  return true;
}

// Función para obtener todos los números primos hasta n
function obtenerPrimos(n) {
  const primos = [];
  for (let i = 2; i <= n; i++) {
    if (esPrimo(i)) {
      primos.push(i);
    }
  }
  return primos;
}

// GET /primos/:number
app.get('/primos/:number', (req, res) => {
  const number = parseInt(req.params.number);
  
  if (isNaN(number) || number < 0) {
    return res.status(400).json({ 
      error: 'Número incorrecto',
      example: 'GET /primos/21'
    });
  }
  
  const primos = obtenerPrimos(number);
  res.json({ 
    input: number, 
    primos: primos,
    cantidad: primos.length,
    operation: 'números primos hasta ' + number
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
    usage: 'GET /primos/:number',
    example: 'GET /primos/21'
  });
});

const server = app.listen(PORT, () => {
  console.log(`Server running at http://0.0.0.0:${PORT}/`);
  console.log(`Try: curl http://localhost:${PORT}/primos/21`);
});

process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
