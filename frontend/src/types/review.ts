/**
 * ReviewLoadInput carries the minimum fields needed to load a PR review.
 * Passed from PRPage to ReviewPage via App.vue navigation state.
 */
export interface ReviewLoadInput {
  owner: string
  repo: string
  number: number
  title: string
}
