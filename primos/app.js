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

// Función para simular consumo de CPU
function consumirCPU(iteraciones) {
  let resultado = 0;
  for (let i = 0; i < iteraciones; i++) {
    resultado += Math.sqrt(i) * Math.sin(i);
  }
  return resultado;
}

// Función para simular consumo de memoria
function consumirMemoria(mb) {
  const bytes = mb * 1024 * 1024;
  const array = new Array(bytes / 4); // Array de floats (4 bytes cada uno)
  for (let i = 0; i < array.length; i++) {
    array[i] = Math.random();
  }
  return array.length;
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
    operation: 'números primos hasta ' + number,
    host: HOSTNAME
  });
});

app.get('/load/:cpu/:mem', (req, res) => {
  const cpu = parseInt(req.params.cpu) || 1000000; // Iteraciones por defecto
  const mem = parseInt(req.params.mem) || 10; // MB por defecto
  
  if (cpu < 0 || mem < 0) {
    return res.status(400).json({ 
      error: 'Parámetros incorrectos',
      example: 'GET /load/1000000/10'
    });
  }
  
  // Consumir CPU
  const cpuResult = consumirCPU(cpu);
  
  // Consumir memoria
  const memResult = consumirMemoria(mem);
  
  res.json({ 
    cpu_iterations: cpu,
    cpu_result: cpuResult,
    mem_mb: mem,
    mem_elements: memResult,
    operation: `consumir ${cpu} iteraciones CPU y ${mem} MB memoria`,
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
