/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export class DaytonaError extends Error {
  public static fromError(error: Error): DaytonaError {
    return new DaytonaError(error.message, {
      cause: error.cause,
    })
  }

  public static fromString(error: string, options?: { cause?: Error }): DaytonaError {
    return DaytonaError.fromError(new Error(error, options))
  }
}
