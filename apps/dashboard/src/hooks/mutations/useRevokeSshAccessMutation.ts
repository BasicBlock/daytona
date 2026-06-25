/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useApi } from '@/hooks/useApi'
import { useMutation } from '@tanstack/react-query'

interface RevokeSshAccessVariables {
  sandboxId: string
  token: string
}

export const useRevokeSshAccessMutation = () => {
  const { sandboxApi } = useApi()

  return useMutation({
    mutationFn: async ({ sandboxId, token }: RevokeSshAccessVariables) => {
      await sandboxApi.revokeSshAccess(sandboxId, token)
    },
  })
}
