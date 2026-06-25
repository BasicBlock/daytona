/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { CreateDockerRegistry, DockerRegistry } from '@daytona/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../queries/queryKeys'
import { useApi } from '../useApi'

export interface CreateRegistryMutationVariables {
  registry: CreateDockerRegistry
}

export const useCreateRegistryMutation = () => {
  const { dockerRegistryApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<DockerRegistry, unknown, CreateRegistryMutationVariables>({
    mutationFn: async ({ registry }) => {
      const response = await dockerRegistryApi.createRegistry(registry)
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.registries.list() })
    },
  })
}
