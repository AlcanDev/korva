import { useMutation } from '@tanstack/react-query'
import { adminPost } from './_fetch'

// Fase 6.2 — wrapper TanStack Query para `/admin/export/obsidian`. Una sola
// mutación: el endpoint escribe en el filesystem del host (path provisto por
// el operador) y devuelve un resumen con los contadores.

export interface ObsidianExportRequest {
  out: string
  project?: string
  type?: string
}

export interface ObsidianExportResult {
  out_dir: string
  file_count: number
  project_count: number
  by_project: Record<string, number>
  by_type: Record<string, number>
  generated_at: string
}

export function useExportObsidian() {
  return useMutation({
    mutationFn: (body: ObsidianExportRequest) =>
      adminPost<ObsidianExportResult>('/admin/export/obsidian', body),
  })
}
