/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import React, { useMemo } from 'react'

import { CommandPalette, useRegisterCommands, type CommandConfig } from '@/components/CommandPalette'
import { Sidebar } from '@/components/Sidebar'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { Toaster } from '@/components/ui/sonner'
import { DAYTONA_DOCS_URL, DAYTONA_SLACK_URL } from '@/constants/ExternalLinks'
import { useTheme } from '@/contexts/ThemeContext'
import { useConfig } from '@/hooks/useConfig'
import { cn } from '@/lib/utils'
import { SlackLogoIcon } from '@phosphor-icons/react'
import { BookOpen, SunMoon } from 'lucide-react'

function useDashboardCommands() {
  const { theme, setTheme } = useTheme()

  const helpCommands: CommandConfig[] = useMemo(
    () => [
      {
        id: 'open-slack',
        label: 'Open Slack',
        icon: <SlackLogoIcon className="w-4 h-4" />,
        onSelect: () => window.open(DAYTONA_SLACK_URL, '_blank'),
      },
      {
        id: 'open-docs',
        label: 'Open Docs',
        icon: <BookOpen className="w-4 h-4" />,
        onSelect: () => window.open(DAYTONA_DOCS_URL, '_blank'),
      },
    ],
    [],
  )
  useRegisterCommands(helpCommands, { groupId: 'help', groupLabel: 'Help', groupOrder: 2 })

  const globalCommands: CommandConfig[] = useMemo(
    () => [
      {
        id: 'toggle-theme',
        label: 'Toggle Theme',
        icon: <SunMoon className="w-4 h-4" />,
        onSelect: () => setTheme(theme === 'dark' ? 'light' : 'dark'),
      },
    ],
    [theme, setTheme],
  )
  useRegisterCommands(globalCommands, { groupId: 'global', groupLabel: 'Global', groupOrder: 5 })
}

type DashboardProps = {
  children: React.ReactNode
}

const Dashboard: React.FC<DashboardProps> = ({ children }) => {
  const config = useConfig()

  useDashboardCommands()

  const isBannerVisible = false

  return (
    <div className="relative w-full">
      <SidebarProvider isBannerVisible={isBannerVisible} defaultOpen={true}>
        <Sidebar isBannerVisible={isBannerVisible} version={config.version} />
        <SidebarInset className="overflow-y-auto">
          <div
            className={cn('w-full min-h-screen overscroll-none', {
              'md:pt-12': isBannerVisible,
            })}
          >
            {children}
            <CommandPalette />
          </div>
        </SidebarInset>
        <Toaster />
      </SidebarProvider>
    </div>
  )
}

export default Dashboard
