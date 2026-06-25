/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiClient } from '@/api/apiClient'
import { ApiContext } from '@/contexts/ApiContext'
import { useConfig } from '@/hooks/useConfig'
import { useMemo } from 'react'

export const ApiProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const config = useConfig()
  const api = useMemo(() => new ApiClient(config), [config])

  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>
}
