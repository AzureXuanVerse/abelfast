<script lang="ts">
  // The login screen uses none of the game's art or fonts: they come from
  // its bundles, behind the session.
  import { api, ApiError } from '../lib/api'

  let { onlogin }: { onlogin: () => void } = $props()
  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    busy = true
    error = ''
    try {
      await api('POST', '/login', { password })
      onlogin()
    } catch (err) {
      error = err instanceof ApiError && err.status === 401 ? 'Wrong password' : (err as Error).message
    } finally {
      busy = false
    }
  }
</script>

<main class="grid min-h-screen place-items-center px-4 font-sans">
  <form onsubmit={submit} class="flex w-full max-w-sm flex-col gap-4 rounded bg-card p-8 text-ink shadow-[0_8px_24px_rgb(0_0_0/0.35)]">
    <h1 class="m-0 text-2xl font-extrabold">Belfast back-office</h1>
    <label class="ip-lab">
      Password
      <input type="password" class="ip-field font-sans" autocomplete="current-password" bind:value={password} required />
    </label>
    {#if error}<p role="alert" class="m-0 text-sm text-red">{error}</p>{/if}
    <button type="submit" class="ip-btn font-sans" disabled={busy}>Log in</button>
  </form>
</main>
