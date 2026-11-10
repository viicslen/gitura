import { describe, it, expect } from 'vitest'
import coderabbitRateLimit from './__fixtures__/coderabbit-rate-limit.md?raw'
import { markdownPreview, renderGitHubMarkdown, setTaskListItem } from './markdown'

function render(md: string, opts = {}): Document {
  return new DOMParser().parseFromString(renderGitHubMarkdown(md, opts), 'text/html')
}

describe('renderGitHubMarkdown', () => {
  it.each([
    ['NOTE', 'Note'],
    ['TIP', 'Tip'],
    ['IMPORTANT', 'Important'],
    ['WARNING', 'Warning'],
    ['CAUTION', 'Caution'],
  ])('turns > [!%s] into a titled alert', (marker, title) => {
    const doc = render(`> [!${marker}]\n> Body text.`)
    const alert = doc.querySelector(`.markdown-alert.markdown-alert-${marker.toLowerCase()}`)
    expect(alert).not.toBeNull()
    expect(alert?.querySelector('.markdown-alert-title')?.textContent?.trim()).toBe(title)
    expect(alert?.textContent).toContain('Body text.')
    expect(alert?.textContent).not.toContain(`[!${marker}]`)
    expect(doc.querySelector('blockquote')).toBeNull()
  })

  it('leaves ordinary blockquotes alone', () => {
    const doc = render('> just a quote')
    expect(doc.querySelector('blockquote')?.textContent?.trim()).toBe('just a quote')
    expect(doc.querySelector('.markdown-alert')).toBeNull()
  })

  it('routes <img src> and <source srcset> through the image proxy', () => {
    const doc = render(
      '<picture><source media="(prefers-color-scheme: dark)" srcset="https://storage.googleapis.com/a/dark.svg">'
      + '<img src="https://storage.googleapis.com/a/light.svg" alt="x"></picture>',
    )
    expect(doc.querySelector('source')?.getAttribute('srcset'))
      .toBe('/__markdown-image?url=https%3A%2F%2Fstorage.googleapis.com%2Fa%2Fdark.svg')
    expect(doc.querySelector('img')?.getAttribute('src'))
      .toBe('/__markdown-image?url=https%3A%2F%2Fstorage.googleapis.com%2Fa%2Flight.svg')
  })

  it('marks task-list items so they render as checkboxes', () => {
    const doc = render('- [ ] todo\n- [x] done')
    expect(doc.querySelector('ul.contains-task-list')).not.toBeNull()
    expect(doc.querySelectorAll('li.task-list-item input[type="checkbox"]')).toHaveLength(2)
  })

  it('enables and numbers task checkboxes only when asked', () => {
    expect(render('- [ ] a').querySelector('input')?.hasAttribute('disabled')).toBe(true)
    const boxes = Array.from(render('- [ ] a\n- [x] b', { tasks: true }).querySelectorAll('input'))
    expect(boxes.map((b) => [b.disabled, b.dataset.taskIndex])).toEqual([[false, '0'], [false, '1']])
  })

  it('renders the CodeRabbit rate-limit comment like GitHub does', () => {
    const doc = render(coderabbitRateLimit)
    const alert = doc.querySelector('.markdown-alert-warning')
    expect(alert).not.toBeNull()
    expect(alert?.querySelector('h2')?.textContent).toBe('Review limit reached')
    expect(doc.body.textContent).not.toContain('[!WARNING]')
    for (const s of Array.from(doc.querySelectorAll('source[srcset]'))) {
      expect(s.getAttribute('srcset')).toMatch(/^\/__markdown-image\?/)
    }
  })
})

describe('setTaskListItem', () => {
  it('flips the box the renderer shows, skipping look-alikes in code and links', () => {
    const md = [
      'Intro [x] link-ish text',
      '',
      '    - [ ] indented code',
      '',
      '```',
      '- [ ] fenced code',
      '```',
      '',
      '- [ ] first',
      '> - [x] quoted',
    ].join('\r\n')
    expect(setTaskListItem(md, 0, true)).toBe(md.replace('- [ ] first', '- [x] first'))
    expect(setTaskListItem(md, 1, false)).toBe(md.replace('> - [x] quoted', '> - [ ] quoted'))
  })

  it('returns null when the box is missing or already in that state', () => {
    expect(setTaskListItem('- [x] done', 0, true)).toBeNull()
    expect(setTaskListItem('- [ ] todo', 3, true)).toBeNull()
  })
})

describe('markdownPreview', () => {
  it('strips tags, comments and markdown markers into one line', () => {
    expect(markdownPreview('<!-- bot -->**Fix** this\n\n<details>`code`</details>')).toBe('Fix this code')
  })

  it('does not leave tags spliced together from nested fragments', () => {
    expect(markdownPreview('<scr<script>ipt>alert(1)')).not.toContain('<script')
    expect(markdownPreview('<!<!--- -->-- x -->')).not.toContain('<!--')
  })
})
