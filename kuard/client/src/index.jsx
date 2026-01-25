import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './app'

if (process.env.NODE_ENV !== 'production') {
  console.log('Looks like we are in development mode!');
}

const root = createRoot(document.getElementById("root"));
root.render(<App page={pageContext}/>);
