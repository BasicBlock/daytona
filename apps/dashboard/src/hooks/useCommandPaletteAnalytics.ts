/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export function useCommandPaletteAnalytics() {
  return {
    trackOpened: (..._args: unknown[]) => undefined,
    trackCommandExecuted: (..._args: unknown[]) => undefined,
    trackPageNavigated: (..._args: unknown[]) => undefined,
    trackSearched: (..._args: unknown[]) => undefined,
  }
}
