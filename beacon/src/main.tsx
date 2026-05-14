import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import { consumeOIDCSessionFromURL } from '@/auth/oidc'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

// Phase 15.D — consume any OIDC session token left in the URL hash by
// the vault's /auth/oidc/callback. Strips the fragment in the same
// tick so the token doesn't leak via history / Referer / screenshots.
consumeOIDCSessionFromURL()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
