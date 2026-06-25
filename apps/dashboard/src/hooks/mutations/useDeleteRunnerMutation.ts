/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Runner } from '@daytona/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'
import { mutationKeys } from './mutationKeys'

export interface DeleteRunnerMutationVariables {
  runnerId: string
}

interface UseDeleteRunnerMutationOptions {
  invalidateOnSuccess?: boolean
}

export const useDeleteRunnerMutation = ({ invalidateOnSuccess = true }: UseDeleteRunnerMutationOptions = {}) => {
  const { runnersApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<void, unknown, DeleteRunnerMutationVariables>({
    mutationKey: mutationKeys.runners.remove(),
    mutationFn: async ({ runnerId }) => {
      await runnersApi.deleteRunner(runnerId)
    },
    onSuccess: async (_data, { runnerId }) => {
      const queryKey = queryKeys.runners.list()
      queryClient.setQueriesData<Runner[]>({ queryKey }, (previousRunners) => {
        if (!previousRunners) return previousRunners

        return previousRunners.filter((runner) => runner.id !== runnerId)
      })

      if (invalidateOnSuccess) {
        await queryClient.invalidateQueries({ queryKey })
      }
    },
  })
}
