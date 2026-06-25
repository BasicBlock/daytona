/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Command,
  CommandCheckboxItem,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandInputButton,
  CommandList,
} from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Loader2, X } from 'lucide-react'
import { FacetedFilterOption } from '../types'

interface TargetFilterProps {
  value: string[]
  onFilterChange: (value: string[] | undefined) => void
  options?: FacetedFilterOption[]
  isLoading?: boolean
}

export function TargetFilterIndicator({ value, onFilterChange, options, isLoading }: TargetFilterProps) {
  const selectedTargetLabels = value
    .map((v) => options?.find((r) => r.value === v)?.label)
    .filter(Boolean)
    .join(', ')

  return (
    <div className="flex items-center h-6 gap-0.5 rounded-sm border border-border bg-muted/80 hover:bg-muted/50 text-sm">
      <Popover>
        <PopoverTrigger className="max-w-[160px] overflow-hidden text-ellipsis whitespace-nowrap text-muted-foreground px-2">
          Target:{' '}
          <span className="text-primary font-medium">
            {selectedTargetLabels.length > 0 ? selectedTargetLabels : 'All'}
          </span>
        </PopoverTrigger>

        <PopoverContent className="p-0 w-72" align="start">
          <TargetFilter value={value} onFilterChange={onFilterChange} options={options} isLoading={isLoading} />
        </PopoverContent>
      </Popover>

      <button className="h-6 w-5 p-0 border-0 hover:text-muted-foreground" onClick={() => onFilterChange(undefined)}>
        <X className="h-3 w-3" />
      </button>
    </div>
  )
}

export function TargetFilter({ value, onFilterChange, options, isLoading }: TargetFilterProps) {
  return (
    <Command>
      <CommandInput placeholder="Search..." className="">
        <CommandInputButton onClick={() => onFilterChange(undefined)}>Clear</CommandInputButton>
      </CommandInput>
      <CommandList>
        {isLoading ? (
          <div className="flex items-center justify-center py-6">
            <Loader2 className="h-4 w-4 animate-spin mr-2" />
            <span className="text-sm text-muted-foreground">Loading targets...</span>
          </div>
        ) : (
          <>
            <CommandEmpty>No targets found.</CommandEmpty>
            <CommandGroup>
              {options?.map((target) => (
                <CommandCheckboxItem
                  checked={value.includes(target.value)}
                  key={target.value}
                  onSelect={() => {
                    const newValue = value.includes(target.value)
                      ? value.filter((v) => v !== target.value)
                      : [...value, target.value]
                    onFilterChange(newValue.length > 0 ? newValue : undefined)
                  }}
                >
                  {target.label}
                </CommandCheckboxItem>
              ))}
            </CommandGroup>
          </>
        )}
      </CommandList>
    </Command>
  )
}
