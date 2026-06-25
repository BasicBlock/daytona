/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { DockerRegistry, UpdateDockerRegistry } from '@daytona/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface UpdateRegistryMutationVariables {
  registryId: string
  registry: UpdateDockerRegistry
}

export const useUpdateRegistryMutation = () => {
  const { dockerRegistryApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<DockerRegistry, unknown, UpdateRegistryMutationVariables>({
    mutationFn: async ({ registryId, registry }) => {
      const response = await dockerRegistryApi.updateRegistry(registryId, registry)
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.registries.list() })
    },
  })
}
