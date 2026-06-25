/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import type { FileInfo } from '@daytona/toolbox-api-client'
import type { Buffer } from 'buffer'

export type PreviewKind = 'binary' | 'image' | 'text'

export type FileUpload = {
  source: Buffer | ArrayBuffer | Uint8Array | Blob
  destination: string
}

export type SandboxInstance = {
  fs: {
    createFolder: (path: string, mode: string) => Promise<void>
    deleteFile: (path: string, recursive?: boolean) => Promise<void>
    downloadFile: (path: string) => Promise<ArrayBuffer>
    getFileDetails: (path: string) => Promise<FileInfo>
    listFiles: (path: string) => Promise<FileInfo[]>
    moveFiles: (source: string, destination: string) => Promise<void>
    searchFiles: (path: string, pattern: string) => Promise<{ files: string[] }>
    uploadFiles: (files: FileUpload[]) => Promise<void>
  }
  id: string
}

export type SandboxFileSystemNode = {
  group: string
  id: string
  isDir: boolean
  modTime: string
  mode: string
  name: string
  owner: string
  path: string
  permissions: string
  size: number
}
