/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { queryKeys } from '@/hooks/queries/queryKeys'
import { useMutation, useQueryClient } from '@tanstack/react-query'

interface PauseSandboxVariables {
  sandboxId: string
}

interface UsePauseSandboxMutationOptions {
  invalidate?: boolean
}

export const usePauseSandboxMutation = ({ invalidate = true }: UsePauseSandboxMutationOptions = {}) => {
  const { sandboxApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ sandboxId }: PauseSandboxVariables) => {
      await sandboxApi.pauseSandbox(sandboxId)
    },
    onSuccess: (_, { sandboxId }) => {
      if (!invalidate) {
        return
      }

      queryClient.invalidateQueries({
        queryKey: queryKeys.sandboxes.detail(sandboxId),
      })
    },
  })
}
