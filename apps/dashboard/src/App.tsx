/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Suspense } from 'react'
import {
  createBrowserRouter,
  Navigate,
  Outlet,
  redirect,
  useLocation,
  useNavigation,
  useRouteError,
} from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { BannerProvider } from './components/Banner'
import { CommandPaletteProvider } from './components/CommandPalette'
import { ErrorBoundaryFallback } from './components/ErrorBoundaryFallback'
import LoadingFallback from './components/LoadingFallback'
import { LoadingFallbackContent } from './components/LoadingFallbackContent'
import { DAYTONA_DOCS_URL, DAYTONA_SLACK_URL } from './constants/ExternalLinks'
import { getRouteSubPath, RoutePath, trimLeadingSlash } from './enums/RoutePath'
import Dashboard from './pages/Dashboard'
import NotFound from './pages/NotFound'
import { ApiProvider } from './providers/ApiProvider'
import { lazyRoutes } from './routes'

function normalizeRouteError(error: unknown) {
  if (error instanceof Error) {
    return error
  }

  if (typeof error === 'string') {
    return new Error(error)
  }

  return new Error('Unknown route error')
}

function RouteErrorFallback() {
  const error = useRouteError()

  return (
    <ErrorBoundaryFallback error={normalizeRouteError(error)} resetErrorBoundary={() => window.location.reload()} />
  )
}

function DashboardOutlet() {
  const location = useLocation()
  const navigation = useNavigation()
  const isRouteLoading = navigation.state === 'loading' && navigation.location?.pathname !== location.pathname

  return (
    <Suspense fallback={<LoadingFallback source="dashboard-suspense" />}>
      <ApiProvider>
        <CommandPaletteProvider>
          <BannerProvider>
            <Dashboard>
              {isRouteLoading ? (
                <div className="flex min-h-screen w-full items-center justify-center bg-background p-6">
                  <LoadingFallbackContent source="route-navigation" />
                </div>
              ) : (
                <Outlet />
              )}
            </Dashboard>
          </BannerProvider>
        </CommandPaletteProvider>
      </ApiProvider>
    </Suspense>
  )
}

function DashboardIndexRedirect() {
  const location = useLocation()

  return <Navigate to={`${getRouteSubPath(RoutePath.SANDBOXES)}${location.search}`} replace />
}

const router = createBrowserRouter([
  {
    path: RoutePath.LANDING,
    hydrateFallbackElement: <LoadingFallback source="app-root-hydrate" />,
    errorElement: <RouteErrorFallback />,
    children: [
      { index: true, element: <Navigate to={RoutePath.DASHBOARD} replace /> },
      { path: trimLeadingSlash(RoutePath.DOCS), loader: () => redirect(DAYTONA_DOCS_URL) },
      { path: trimLeadingSlash(RoutePath.SLACK), loader: () => redirect(DAYTONA_SLACK_URL) },
      {
        path: trimLeadingSlash(RoutePath.DASHBOARD),
        element: <DashboardOutlet />,
        children: [
          { index: true, element: <DashboardIndexRedirect /> },
          { path: getRouteSubPath(RoutePath.SANDBOXES), lazy: lazyRoutes.Sandboxes },
          { path: getRouteSubPath(RoutePath.SANDBOX_DETAILS), lazy: lazyRoutes.SandboxDetails },
          { path: getRouteSubPath(RoutePath.SNAPSHOTS), lazy: lazyRoutes.Snapshots },
          { path: getRouteSubPath(RoutePath.REGISTRIES), lazy: lazyRoutes.Registries },
          { path: getRouteSubPath(RoutePath.VOLUMES), lazy: lazyRoutes.Volumes },
          { path: getRouteSubPath(RoutePath.RUNNERS), lazy: lazyRoutes.Runners },
        ],
      },
      { path: '*', element: <NotFound /> },
    ],
  },
])

function App() {
  return <RouterProvider router={router} />
}

export default App
