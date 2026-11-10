/** Identifies a GitHub PR plus the header fields shown before the review loads. */
export interface PRRef {
  owner: string
  repo: string
  number: number
  title: string
  state?: string
  is_draft?: boolean
  html_url?: string
}
