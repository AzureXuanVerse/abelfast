// A number edited in place: one field open at a time, keyed by what it
// edits. Its form saves on Enter, Escape closes it.
export class InlineEdit {
  #reset?: () => unknown
  #onstop?: () => void
  // the key of the field open; it closes when reset's value changes
  editing = $derived.by<string | null>(() => (this.#reset?.(), null))
  value = $state(0)
  // a number edited with the first (a best score)
  second = $state(0)

  constructor({ reset, onstop }: { reset?: () => unknown; onstop?: () => void } = {}) {
    this.#reset = reset
    this.#onstop = onstop
  }

  start = (key: string, value: number, second = 0) => {
    this.value = value
    this.second = second
    this.editing = key
  }
  stop = () => {
    this.editing = null
    this.#onstop?.()
  }
  escape = (event: KeyboardEvent) => {
    if (event.key === 'Escape') this.stop()
  }
  // submit closes the field for a form that saves by itself
  submit = (event: SubmitEvent) => {
    event.preventDefault()
    this.stop()
  }
  // save closes the field and applies its value when it changed
  save = (event: SubmitEvent, current: number, apply: (value: number) => void) => {
    this.submit(event)
    if (this.value !== current) apply(this.value)
  }
}
