/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

type NotificationSocket = {
  on: (event: string, handler: (...args: any[]) => void) => void
  off: (event: string, handler: (...args: any[]) => void) => void
}

export function useNotificationSocket(): { notificationSocket: NotificationSocket | null } {
  return { notificationSocket: null }
}
