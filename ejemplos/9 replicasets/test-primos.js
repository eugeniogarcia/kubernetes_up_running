import http from 'k6/http';
import { sleep, check } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";


// configuramos una rampa de usuarios virtuales
export const options = {
  stages: [
    { duration: '1m30s', target: 8 }, // ramp-up a 8 usuarios en 2 minutos 
    { duration: '30s', target: 8 }, 
    { duration: '2m', target: 2 }, 
    { duration: '1m', target: 2 }, 
    { duration: '40s', target: 1 }, 
    { duration: '20s', target: 0 }, 
  ],
};

export default function() {
  let res = http.get('http://gz.com/load/1/10'); // 1M interaciones CPU, 10M memoria
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(0.5);
}

// customiza el informe generado por k6
export function handleSummary(data) {
  return {
    'resultados/k6/reports/replicasets_report.html': htmlReport(data), // crea el informe indicado en la key a partir del resultado (que se pasa en data)
    'resultados/k6/reports/replicasets_report.json': JSON.stringify(data),
  };
}