/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { queryKeys } from '@/hooks/queries/queryKeys'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { mutationKeys } from './mutationKeys'

interface RecoverSandboxVariables {
  sandboxId: string
}

interface UseRecoverSandboxMutationOptions {
  invalidate?: boolean
}

export const useRecoverSandboxMutation = ({ invalidate = true }: UseRecoverSandboxMutationOptions = {}) => {
  const { sandboxApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation({
    mutationKey: mutationKeys.sandboxes.recover(),
    mutationFn: async ({ sandboxId }: RecoverSandboxVariables) => {
      await sandboxApi.recoverSandbox(sandboxId)
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
