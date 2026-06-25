/**
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

/**
 * Enum for all route paths in the application
 * Use this for consistent route references across the app
 */
export enum RoutePath {
  // Main routes
  LANDING = '/',
  DASHBOARD = '/dashboard',
  DOCS = '/docs',
  SLACK = '/slack',

  // Dashboard sub-routes
  SANDBOXES = '/dashboard/sandboxes',
  SNAPSHOTS = '/dashboard/snapshots',
  REGISTRIES = '/dashboard/registries',
  VOLUMES = '/dashboard/volumes',
  RUNNERS = '/dashboard/runners',

  // Sandboxes
  SANDBOX_DETAILS = '/dashboard/sandboxes/:sandboxId',
}

/**
 * Returns only the path segment for dashboard sub-routes
 * Useful for nested routes under the dashboard
 */
export const getRouteSubPath = (path: RoutePath): string => {
  return path.replace('/dashboard/', '')
}

/**
 * Returns a route path without the leading slash.
 * Useful for nested route definitions that expect relative paths.
 */
export const trimLeadingSlash = (path: RoutePath): string => {
  return path.replace(/^\//, '')
}
