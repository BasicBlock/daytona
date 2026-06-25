/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { CreateResourceButton } from '@/components/CreateResourceButton'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Spinner } from '@/components/ui/spinner'
import { useCreateSnapshotMutation } from '@/hooks/mutations/useCreateSnapshotMutation'
import { handleApiError } from '@/lib/error-handling'
import { GPU_TYPE_LABELS } from '@/lib/gpu-types'
import { imageNameSchema } from '@/lib/schema'
import { cn } from '@/lib/utils'
import type { SnapshotDto } from '@daytona/api-client'
import { GpuType } from '@daytona/api-client'
import { useForm } from '@tanstack/react-form'
import { Ref, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'
import { ScrollArea } from '../ui/scroll-area'

const IMAGE_NAME_REGEX = /^[a-zA-Z0-9_.\-:]+(\/[a-zA-Z0-9_.\-:]+)*(@sha256:[a-f0-9]{64})?$/

const snapshotNameSchema = z
  .string()
  .min(1, 'Snapshot name is required')
  .refine((name) => IMAGE_NAME_REGEX.test(name), 'Only letters, digits, dots, colons, slashes and dashes are allowed')

const SELECTABLE_GPU_TYPES = (Object.values(GpuType) as GpuType[]).filter((t) => t !== GpuType.UNKNOWN_DEFAULT_OPEN_API)

const formSchema = z.object({
  name: snapshotNameSchema,
  imageName: imageNameSchema,
  entrypoint: z.string().optional(),
  cpu: z.number().min(1).optional(),
  memory: z.number().min(1).optional(),
  disk: z.number().min(1).optional(),
  gpu: z.boolean().optional(),
  gpuType: z.nativeEnum(GpuType).optional(),
})

type FormValues = z.infer<typeof formSchema>

const defaultValues: FormValues = {
  name: '',
  imageName: '',
  entrypoint: '',
  cpu: undefined,
  memory: undefined,
  disk: undefined,
  gpu: false,
  gpuType: undefined,
}

export const CreateSnapshotSheet = ({
  className,
  onSnapshotCreated,
  ref,
}: {
  className?: string
  onSnapshotCreated?: (snapshot: SnapshotDto) => void
  ref?: Ref<{ open: () => void }>
}) => {
  const [open, setOpen] = useState(false)

  const { reset: resetCreateSnapshotMutation, ...createSnapshotMutation } = useCreateSnapshotMutation()
  const formRef = useRef<HTMLFormElement>(null)
  const formDefaultValues = useMemo<FormValues>(() => defaultValues, [])

  useImperativeHandle(ref, () => ({
    open: () => setOpen(true),
  }))

  const form = useForm({
    defaultValues: formDefaultValues,
    validators: {
      onSubmit: formSchema,
    },
    onSubmitInvalid: () => {
      const form = formRef.current
      if (!form) return
      const invalidInput = form.querySelector('[aria-invalid="true"]') as HTMLInputElement | null
      if (invalidInput) {
        invalidInput.scrollIntoView({ behavior: 'smooth', block: 'center' })
        invalidInput.focus()
      }
    },
    onSubmit: async ({ value }) => {
      const trimmedEntrypoint = value.entrypoint?.trim()

      try {
        const snapshot = await createSnapshotMutation.mutateAsync({
          snapshot: {
            name: value.name.trim(),
            imageName: value.imageName.trim(),
            entrypoint: trimmedEntrypoint ? trimmedEntrypoint.split(' ') : undefined,
            cpu: value.cpu,
            memory: value.memory,
            disk: value.disk,
            gpu: value.gpu ? 1 : undefined,
            gpuType: value.gpu && value.gpuType ? [value.gpuType] : undefined,
          },
        })

        toast.success(`Creating snapshot ${value.name.trim()}`)
        onSnapshotCreated?.(snapshot)
        setOpen(false)
      } catch (error) {
        handleApiError(error, 'Failed to create snapshot')
      }
    },
  })
  const { reset: resetForm } = form

  const resetState = useCallback(() => {
    resetForm(formDefaultValues)
    resetCreateSnapshotMutation()
  }, [formDefaultValues, resetForm, resetCreateSnapshotMutation])

  useEffect(() => {
    if (open) {
      resetState()
    }
  }, [open, resetState])

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <CreateResourceButton resource="Snapshot" />
      </SheetTrigger>
      <SheetContent className={cn('w-dvw sm:w-[500px] p-0 flex flex-col gap-0', className)}>
        <SheetHeader className="border-b border-border p-4 px-5 items-center flex text-left flex-row">
          <SheetTitle>Create Snapshot</SheetTitle>
          <SheetDescription className="sr-only">
            Register a new snapshot to be used for spinning up sandboxes in your workspace.
          </SheetDescription>
        </SheetHeader>
        <ScrollArea fade="mask" className="flex-1 min-h-0">
          <form
            ref={formRef}
            id="create-snapshot-form"
            className="gap-6 flex flex-col p-5"
            onSubmit={(e) => {
              e.preventDefault()
              e.stopPropagation()
              form.handleSubmit()
            }}
          >
            <form.Field name="name">
              {(field) => {
                const isInvalid = field.state.meta.isTouched && !field.state.meta.isValid
                return (
                  <Field data-invalid={isInvalid}>
                    <FieldLabel htmlFor={field.name}>Snapshot Name</FieldLabel>
                    <Input
                      aria-invalid={isInvalid}
                      id={field.name}
                      name={field.name}
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="ubuntu-4vcpu-8ram-100gb"
                    />
                    <FieldDescription>
                      The name you will use in your client app (SDK, CLI) to reference the snapshot.
                    </FieldDescription>
                    {field.state.meta.errors.length > 0 && field.state.meta.isTouched && (
                      <FieldError errors={field.state.meta.errors} />
                    )}
                  </Field>
                )
              }}
            </form.Field>

            <form.Field name="imageName">
              {(field) => {
                const isInvalid = field.state.meta.isTouched && !field.state.meta.isValid
                return (
                  <Field data-invalid={isInvalid}>
                    <FieldLabel htmlFor={field.name}>Image</FieldLabel>
                    <Input
                      aria-invalid={isInvalid}
                      id={field.name}
                      name={field.name}
                      value={field.state.value}
                      onBlur={field.handleBlur}
                      onChange={(e) => field.handleChange(e.target.value)}
                      placeholder="ubuntu:22.04"
                    />
                    <FieldDescription>
                      Must include either a tag (e.g., ubuntu:22.04) or a digest. The tag "latest" is not allowed.
                    </FieldDescription>
                    {field.state.meta.errors.length > 0 && field.state.meta.isTouched && (
                      <FieldError errors={field.state.meta.errors} />
                    )}
                  </Field>
                )
              }}
            </form.Field>

            <div className="flex flex-col gap-2">
              <Label className="text-sm font-medium">Resources</Label>
              <div className="flex flex-col gap-2">
                <form.Field name="cpu">
                  {(field) => (
                    <div className="flex items-center gap-4">
                      <Label htmlFor={field.name} className="w-32 flex-shrink-0">
                        Compute (vCPU):
                      </Label>
                      <Input
                        id={field.name}
                        type="number"
                        className="w-full"
                        min="1"
                        placeholder="1"
                        value={field.state.value ?? ''}
                        onChange={(e) => field.handleChange(parseInt(e.target.value) || undefined)}
                      />
                    </div>
                  )}
                </form.Field>
                <form.Field name="memory">
                  {(field) => (
                    <div className="flex items-center gap-4">
                      <Label htmlFor={field.name} className="w-32 flex-shrink-0">
                        Memory (GiB):
                      </Label>
                      <Input
                        id={field.name}
                        type="number"
                        className="w-full"
                        min="1"
                        placeholder="1"
                        value={field.state.value ?? ''}
                        onChange={(e) => field.handleChange(parseInt(e.target.value) || undefined)}
                      />
                    </div>
                  )}
                </form.Field>
                <form.Field name="disk">
                  {(field) => (
                    <div className="flex items-center gap-4">
                      <Label htmlFor={field.name} className="w-32 flex-shrink-0">
                        Storage (GiB):
                      </Label>
                      <Input
                        id={field.name}
                        type="number"
                        className="w-full"
                        min="1"
                        placeholder="3"
                        value={field.state.value ?? ''}
                        onChange={(e) => field.handleChange(parseInt(e.target.value) || undefined)}
                      />
                    </div>
                  )}
                </form.Field>
                <div className="flex flex-col gap-3">
                  <form.Field name="gpu">
                    {(field) => (
                      <div className="flex items-start gap-2 pt-1">
                        <Checkbox
                          id={field.name}
                          className="mt-0.5"
                          checked={field.state.value ?? false}
                          onCheckedChange={(checked) => {
                            const isChecked = checked === true
                            field.handleChange(isChecked)
                            if (isChecked) {
                              if (!form.getFieldValue('gpuType') && SELECTABLE_GPU_TYPES.length > 0) {
                                form.setFieldValue('gpuType', SELECTABLE_GPU_TYPES[0])
                              }
                            } else {
                              form.setFieldValue('gpuType', undefined)
                            }
                          }}
                        />
                        <div className="flex flex-col gap-1">
                          <Label htmlFor={field.name} className="text-sm font-normal">
                            Allocate GPU
                          </Label>
                          <FieldDescription>
                            Sandboxes created from this snapshot must be ephemeral - set autoDeleteInterval to 0 when
                            creating the sandbox.
                          </FieldDescription>
                        </div>
                      </div>
                    )}
                  </form.Field>
                  <form.Subscribe selector={(state) => state.values.gpu}>
                    {(gpuEnabled) =>
                      gpuEnabled && SELECTABLE_GPU_TYPES.length > 0 ? (
                        <form.Field name="gpuType">
                          {(field) => (
                            <Field>
                              <FieldLabel htmlFor={field.name}>GPU type</FieldLabel>
                              <Select
                                value={field.state.value ?? SELECTABLE_GPU_TYPES[0]}
                                onValueChange={(val) => field.handleChange(val as GpuType)}
                              >
                                <SelectTrigger className="h-8" id={field.name}>
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  {SELECTABLE_GPU_TYPES.map((gpuType) => (
                                    <SelectItem key={gpuType} value={gpuType}>
                                      {GPU_TYPE_LABELS[gpuType] || gpuType}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </Field>
                          )}
                        </form.Field>
                      ) : null
                    }
                  </form.Subscribe>
                </div>
              </div>
              <FieldDescription>
                If not specified, default values will be used (1 vCPU, 1 GiB memory, 3 GiB storage).
              </FieldDescription>
            </div>

            <form.Field name="entrypoint">
              {(field) => (
                <Field>
                  <FieldLabel htmlFor={field.name}>Entrypoint (optional)</FieldLabel>
                  <Input
                    id={field.name}
                    name={field.name}
                    value={field.state.value ?? ''}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    placeholder="sleep infinity"
                  />
                  <FieldDescription>
                    Ensure that the entrypoint is a long running command. If not provided, or if the snapshot does not
                    have an entrypoint, 'sleep infinity' will be used as the default.
                  </FieldDescription>
                </Field>
              )}
            </form.Field>
          </form>
        </ScrollArea>
        <SheetFooter className="border-t border-border p-4 px-5">
          <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <form.Subscribe
            selector={(state) => [state.canSubmit, state.isSubmitting]}
            children={([canSubmit, isSubmitting]) => (
              <Button type="submit" form="create-snapshot-form" variant="default" disabled={!canSubmit || isSubmitting}>
                {isSubmitting && <Spinner />}
                Create
              </Button>
            )}
          />
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
