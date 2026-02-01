import http from 'k6/http';
import { sleep, check } from 'k6';
import { htmlReport } from "https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js";


// configuramos una rampa de usuarios virtuales
export const options = {
  stages: [
    { duration: '30s', target: 20 }, // ramp-up a 20 usuarios en 30 segundos 
    { duration: '1m30s', target: 10 }, // mantenemos 10 usuarios por 1 minuto y medio
    { duration: '20s', target: 0 }, // ramp-down a 0 usuarios en 20 segundos
  ],
};

export default function() {
  let res = http.get('https://quickpizza.grafana.com');
  check(res, { "status is 200": (res) => res.status === 200 });
  sleep(1);
}

// customiza el informe generado por k6
export function handleSummary(data) {
  return {
    'resultados/k6/reports/replicasets_report.html': htmlReport(data), // crea el informe indicado en la key a partir del resultado (que se pasa en data)
    'resultados/k6/reports/replicasets_report.json': JSON.stringify(data),
  };
}