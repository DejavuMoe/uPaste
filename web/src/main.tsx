import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

function App() {
  return (
    <>
      <header><a href="/" aria-label="uPaste home">uPaste</a></header>
      <main>
        <h1>Share text and files</h1>
        <p>The application foundation is ready. Share creation arrives in a later phase.</p>
      </main>
      <footer><small>Phase 0</small></footer>
    </>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode><App /></StrictMode>,
)
