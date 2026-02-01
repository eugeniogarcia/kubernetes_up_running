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

// GET /load/:cpu/:mem
app.get('/load/:cpu/:mem', (req, res) => {
  const cpu = parseInt(req.params.cpu)*1000000 || 1000000; // Iteraciones por defecto
  const mem = parseInt(req.params.mem) || 10; // MB por defecto
  
  if (cpu < 0 || mem < 0) {
    return res.status(400).json({ 
      error: 'Parámetros incorrectos',
      example: 'GET /load/1/10'
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
  const envMultiplier = parseFloat(process.env.MULTIPLIER);
  const multiplier = isNaN(envMultiplier) ? 2 : envMultiplier;
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
