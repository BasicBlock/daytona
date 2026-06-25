/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { NuqsAdapter } from 'nuqs/adapters/react-router/v7'
import React, { Suspense } from 'react'
import ReactDOM from 'react-dom/client'
import { ErrorBoundary } from 'react-error-boundary'
import App from './App'
import { ErrorBoundaryFallback } from './components/ErrorBoundaryFallback'
import LoadingFallback from './components/LoadingFallback'
import { ThemeProvider } from './contexts/ThemeContext'
import './index.css'
import { ConfigProvider } from './providers/ConfigProvider'
import { QueryProvider } from './providers/QueryProvider'

const root = ReactDOM.createRoot(document.getElementById('root') as HTMLElement)

root.render(
  <React.StrictMode>
    <ErrorBoundary FallbackComponent={ErrorBoundaryFallback}>
      <QueryProvider>
        <ThemeProvider>
          <Suspense fallback={<LoadingFallback source="config-suspense" />}>
            <ConfigProvider>
              <NuqsAdapter>
                <App />
              </NuqsAdapter>
            </ConfigProvider>
          </Suspense>
        </ThemeProvider>
      </QueryProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)
