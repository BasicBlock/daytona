/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { queryKeys } from '@/hooks/queries/queryKeys'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { mutationKeys } from './mutationKeys'

interface ArchiveSandboxVariables {
  sandboxId: string
}

interface UseArchiveSandboxMutationOptions {
  invalidate?: boolean
}

export const useArchiveSandboxMutation = ({ invalidate = true }: UseArchiveSandboxMutationOptions = {}) => {
  const { sandboxApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation({
    mutationKey: mutationKeys.sandboxes.archive(),
    mutationFn: async ({ sandboxId }: ArchiveSandboxVariables) => {
      await sandboxApi.archiveSandbox(sandboxId)
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
