import React from 'react'
import ReactDOM from 'react-dom/client'
import { HashRouter } from 'react-router'
import App from './App'
import './index.css'

// HashRouter is used because the Wails asset server has no SPA fallback;
// hash-based routes survive reloads and deep links without 404s.
ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <HashRouter>
      <App />
    </HashRouter>
  </React.StrictMode>,
)
