/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface DeleteRegistryMutationVariables {
  registryId: string
}

export const useDeleteRegistryMutation = () => {
  const { dockerRegistryApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<void, unknown, DeleteRegistryMutationVariables>({
    mutationFn: async ({ registryId }) => {
      await dockerRegistryApi.deleteRegistry(registryId)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.registries.list() })
    },
  })
}
