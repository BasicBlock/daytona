/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export type FileInfo = {
  group?: string
  name: string
  owner?: string
  path: string
  isDir?: boolean
  isDirectory?: boolean
  size?: number
  mode?: string
  modTime?: string
  permissions?: string
  children?: FileInfo[]
  [key: string]: unknown
}

export type ScreenshotResponse = {
  image?: string
  data?: string
  mimeType?: string
  [key: string]: unknown
}
