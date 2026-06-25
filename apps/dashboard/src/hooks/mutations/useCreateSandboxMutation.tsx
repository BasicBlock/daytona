/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { CreateSandbox, Sandbox } from '@daytona/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { getSandboxesQueryKey } from '../queries/useSandboxesQuery'
import { useApi } from '../useApi'

export type CreateSandboxParams = CreateSandbox

export const useCreateSandboxMutation = () => {
  const { sandboxApi } = useApi()
  const queryClient = useQueryClient()

  return useMutation<Sandbox, unknown, CreateSandboxParams>({
    mutationFn: async (params) => {
      const response = await sandboxApi.createSandbox(params)
      return response.data
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: getSandboxesQueryKey() })
    },
  })
}
