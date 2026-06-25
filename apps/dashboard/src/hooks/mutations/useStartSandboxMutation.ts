/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { queryKeys } from '@/hooks/queries/queryKeys'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { mutationKeys } from './mutationKeys'

interface StartSandboxVariables {
  sandboxId: string
}

interface UseStartSandboxMutationOptions {
  invalidate?: boolean
}

export const useStartSandboxMutation = ({ invalidate = true }: UseStartSandboxMutationOptions = {}) => {
  const { sandboxApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation({
    mutationKey: mutationKeys.sandboxes.start(),
    mutationFn: async ({ sandboxId }: StartSandboxVariables) => {
      await sandboxApi.startSandbox(sandboxId)
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
