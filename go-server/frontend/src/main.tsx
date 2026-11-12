import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { LanguageProvider } from './lib/i18n.tsx'
import { FeedbackProvider } from './components/ui/Feedback.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <LanguageProvider>
      <FeedbackProvider>
        <App />
      </FeedbackProvider>
    </LanguageProvider>
  </StrictMode>,
)
