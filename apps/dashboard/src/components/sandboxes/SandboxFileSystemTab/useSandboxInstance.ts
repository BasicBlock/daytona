/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMemo } from 'react'

import { useApi } from '@/hooks/useApi'
import { useConfig } from '@/hooks/useConfig'

import { useSandboxInstanceQuery } from './queries'
import type { FileUpload, SandboxInstance } from './types'

function createFileSystemError(response: Response, body: unknown, path?: string) {
  const errorBody = body && typeof body === 'object' ? (body as Record<string, unknown>) : {}
  const error = new Error(
    typeof errorBody.message === 'string' ? errorBody.message : `Request failed with status ${response.status}`,
  ) as Error & {
    errorCode?: string
    path?: string
    statusCode?: number
  }

  error.errorCode = typeof errorBody.errorCode === 'string' ? errorBody.errorCode : undefined
  error.path = typeof errorBody.path === 'string' ? errorBody.path : path
  error.statusCode = response.status

  return error
}

async function parseResponseBody(response: Response) {
  const contentType = response.headers.get('content-type') ?? ''

  if (contentType.includes('application/json')) {
    return response.json()
  }

  return response.text()
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit, filePath?: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, init)

  if (!response.ok) {
    throw createFileSystemError(response, await parseResponseBody(response).catch(() => undefined), filePath)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return parseResponseBody(response) as Promise<T>
}

function fileSystemUrl(endpoint: string, params: Record<string, string | boolean | undefined>) {
  const searchParams = new URLSearchParams()

  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) {
      searchParams.set(key, String(value))
    }
  }

  const query = searchParams.toString()
  return `/files${endpoint}${query ? `?${query}` : ''}`
}

async function fileUploadToBlob(upload: FileUpload) {
  if (upload.source instanceof Blob) {
    return upload.source
  }

  return new Blob([upload.source])
}

function createSandboxInstance(id: string, toolboxProxyUrl: string): SandboxInstance {
  const baseUrl = `${toolboxProxyUrl.replace(/\/$/, '')}/${encodeURIComponent(id)}`

  return {
    id,
    fs: {
      createFolder: (path, mode) =>
        request(baseUrl, fileSystemUrl('/folder', { path, mode }), { method: 'POST' }, path),
      deleteFile: (path, recursive) =>
        request(baseUrl, fileSystemUrl('', { path, recursive }), { method: 'DELETE' }, path),
      downloadFile: async (path) => {
        const response = await fetch(`${baseUrl}${fileSystemUrl('/download', { path })}`)

        if (!response.ok) {
          throw createFileSystemError(response, await parseResponseBody(response).catch(() => undefined), path)
        }

        return response.arrayBuffer()
      },
      getFileDetails: (path) => request(baseUrl, fileSystemUrl('/info', { path }), undefined, path),
      listFiles: (path) => request(baseUrl, fileSystemUrl('', { path }), undefined, path),
      moveFiles: (source, destination) =>
        request(baseUrl, fileSystemUrl('/move', { source, destination }), { method: 'POST' }, source),
      searchFiles: (path, pattern) => request(baseUrl, fileSystemUrl('/search', { path, pattern })),
      uploadFiles: async (files) => {
        const formData = new FormData()

        await Promise.all(
          files.map(async (file, index) => {
            formData.append(`files[${index}].path`, file.destination)
            formData.append(`files[${index}].file`, await fileUploadToBlob(file), file.destination)
          }),
        )

        await request(baseUrl, '/files/bulk-upload', { method: 'POST', body: formData })
      },
    },
  }
}

export function useSandboxInstance(sandboxId: string) {
  const api = useApi()
  const config = useConfig()

  const client = useMemo(() => {
    return {
      get: async (id: string) => {
        const { data: sandbox } = await api.sandboxApi.getSandbox(id)
        const toolboxProxyUrl = sandbox.toolboxProxyUrl || config.proxyToolboxUrl

        if (!toolboxProxyUrl) {
          throw new Error('Sandbox toolbox URL is not available')
        }

        return createSandboxInstance(id, toolboxProxyUrl)
      },
    }
  }, [api.sandboxApi, config.proxyToolboxUrl])

  return useSandboxInstanceQuery({
    client,
    sandboxId,
  })
}
