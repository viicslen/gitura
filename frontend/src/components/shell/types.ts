export interface ReviewNavMeta {
  owner: string
  repo: string
  number: number
  title: string
  state?: string
  is_draft?: boolean
  html_url?: string
}
