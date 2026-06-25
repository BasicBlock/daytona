/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { useMutation } from '@tanstack/react-query'

interface CreateSshAccessVariables {
  sandboxId: string
  expiresInMinutes: number
}

export const useCreateSshAccessMutation = () => {
  const { sandboxApi } = useApi()

  return useMutation({
    mutationFn: async ({ sandboxId, expiresInMinutes }: CreateSshAccessVariables) => {
      const response = await sandboxApi.createSshAccess(sandboxId, expiresInMinutes)
      return response.data
    },
  })
}
