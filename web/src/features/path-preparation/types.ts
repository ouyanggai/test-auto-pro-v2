export interface PathPreparationJob {
  id: string
  status: 'queued' | 'running' | 'completed' | 'cancelled' | 'failed'
  total: number
  processed: number
  nodeConfigured: number
  dataGenerated: number
  needsAttention: number
  failed: number
  preservedManual: number
  error?: string
  createdAt: string
  updatedAt: string
  completedAt?: string
}

export interface PathPreparationItem {
  id: number
  pathId: number
  sequenceNo: number
  pathName: string
  status: 'pending' | 'running' | 'completed' | 'needs_attention' | 'failed'
  reason: string
  nodeConfigured: boolean
  dataGenerated: boolean
  needsAttention: boolean
  preservedManual: boolean
}

export interface PathPreparationItemPage {
  items: PathPreparationItem[]
  nextCursor?: number
}
