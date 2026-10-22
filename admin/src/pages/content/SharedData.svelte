<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query'
  import Page from '../../components/Page.svelte'
  import Panel from '../../components/Panel.svelte'
  import ContentTabs from '../../components/ContentTabs.svelte'
  import { api } from '../../lib/api'
  import { createWrite, get, keys } from '../../lib/query'
  import { contentKeys, serverDay, serverShort, type Boss, type Shared, type Survey } from '../../lib/content'

  const query = createQuery(() => ({ queryKey: contentKeys.shared, queryFn: get<Shared>('/shared') }))
  const data = $derived(query.data ?? null)

  const offset = $derived(data?.offset ?? 0)
  const markLabel = (mark: number) => (mark === 0 ? 'Defeated' : `${mark / 100}%`)

  const marking = createWrite(
    ({ boss, mark }: { boss: Boss; mark: number }) => api('POST', '/shared/marks/reset', { activity_id: boss.activity_id, boss_id: boss.boss_id, mark }),
    [contentKeys.shared],
    { success: 'Mark reset' },
  )

  function resetMark(boss: Boss, mark: number) {
    if (!confirm(`Reset the ${markLabel(mark)} mark of ${boss.name || 'boss ' + boss.boss_id}?`)) return
    marking.mutate({ boss, mark })
  }

  // those who took it and were playing are kicked: every commander screen
  // and the online lists refetch
  const surveying = createWrite((survey: Survey) => api('POST', `/shared/surveys/${survey.id}/reset`), [contentKeys.shared, ['commander'], keys.online, keys.commanders], {
    success: 'Survey reset',
  })

  function resetSurvey(survey: Survey) {
    if (!confirm(`Let the ${survey.taken} commander${survey.taken === 1 ? '' : 's'} who took ${survey.name || 'survey ' + survey.id} take it again? Those playing are disconnected.`)) return
    surveying.mutate(survey)
  }
</script>

<Page title="Shared data" sub="SERVER-WIDE" back="/events">
  {#snippet actions()}
    {#if data}<span class="text-soft">Game data <span class="text-white">{data.region}</span> <span class="num text-xl text-white">{data.data}</span></span>{/if}
  {/snippet}
  <div class="relative flex items-start gap-6 pt-[18px] pr-10 pb-7 pl-16">
    <ContentTabs />
    <div class="flex min-w-0 flex-1 flex-wrap items-start gap-6">
      <Panel title="Activity bosses" sub="EVERY COMMANDER" class="min-w-0 flex-[1_1_560px]">
        <div class="flex flex-col gap-3 p-3">
          {#each data?.bosses ?? [] as boss (boss.activity_id + ':' + boss.boss_id)}
            <article class={['ip-row flex flex-col gap-3 px-4 py-3.5', boss.open && 'ip-sel']}>
              <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span class={['ip-tag self-center text-white', boss.collab ? 'bg-[#7b5cf0]' : 'bg-red']}>{boss.collab ? 'COLLAB RUSH' : 'BOSS BATTLE'}</span>
                <h3 class="m-0 text-[22px]">{boss.name || 'Boss ' + boss.boss_id}</h3>
                {#if boss.collab}<span class="text-[15px] text-soft">Series <span class="num text-lg text-white">{boss.series}</span></span>{/if}
                {#if boss.open}<span class="text-[15px] text-ok">Running</span>{/if}
                <span class="ml-auto text-sm text-soft">ID <span class="num text-lg text-white">{boss.activity_id}</span></span>
              </div>
              <div class="flex items-center gap-4">
                <span class="ip-sub">HP</span>
                <div class="relative h-3.5 flex-1 overflow-hidden rounded-full bg-black/45">
                  <span class="absolute inset-y-0 left-0 rounded-full bg-red" style:width="{boss.hp / 100}%"></span>
                  {#each [25, 50, 75] as tick (tick)}<span class="absolute inset-y-0 w-0.5 bg-white/30" style:left="{tick}%"></span>{/each}
                </div>
                <span class="num w-16 text-right text-[26px]">{Math.round(boss.hp / 100)}%</span>
              </div>
              <div class="grid grid-cols-4 gap-2 max-[900px]:grid-cols-2">
                {#each boss.marks as m (m.mark)}
                  <div class={['flex min-h-[78px] flex-col gap-1 rounded-[3px] border-t-[3px] bg-black/30 px-2.5 py-1.5', m.reached_at ? 'border-red' : 'border-white/15']}>
                    <span class="text-[15px]">{markLabel(m.mark)}</span>
                    {#if m.reached_at}
                      <span class="num text-lg text-soft">{serverShort(m.reached_at, offset)}</span>
                      <button type="button" class="ip-btn2 mt-auto h-8 px-2 text-[15px]" disabled={marking.isPending} onclick={() => resetMark(boss, m.mark)}>Reset mark</button>
                    {:else}
                      <span class="text-sm text-dim">Not yet</span>
                    {/if}
                  </div>
                {/each}
              </div>
            </article>
          {:else}
            {#if data}<p class="m-0 py-6 text-center text-soft">No boss activity running or marked</p>{:else if query.error}<p class="m-0 py-6 text-center text-bad">{query.error.message}</p>{/if}
          {/each}
        </div>
      </Panel>

      <Panel title="Questionnaires" sub="SURVEYS" class="min-w-[380px] flex-[0_1_510px]">
        <div class="overflow-x-auto p-3">
          <table class="w-full border-separate border-spacing-y-1.5 text-left">
            <thead>
              <tr class="ip-sub">
                <th class="px-3 font-bold">SURVEY</th>
                <th class="px-2 font-bold">TAKEN</th>
                <th class="px-2 font-bold">LATEST</th>
                <th><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {#each data?.surveys ?? [] as survey (survey.id)}
                <tr class="bg-white/6">
                  <td class="rounded-l-[3px] px-3 py-2">
                    <div class="text-[16px] leading-tight">{survey.name || 'Survey'}</div>
                    <div class="flex items-center gap-2">
                      <span class="num text-soft">{survey.id}</span>
                      {#if survey.open}<span class="text-[13px] text-ok">Running</span>{/if}
                    </div>
                  </td>
                  <td class="px-2 py-2 whitespace-nowrap"><span class="num text-2xl">{survey.taken}</span><span class="text-sm text-soft">/{data?.commanders}</span></td>
                  <td class="num px-2 py-2 text-xl whitespace-nowrap text-soft">{survey.latest ? serverDay(survey.latest, offset) : '-'}</td>
                  <td class="rounded-r-[3px] px-3 py-2 text-right">
                    <button type="button" class="ip-btn2 ip-sm" disabled={!survey.taken || surveying.isPending} aria-label="Reset {survey.name}" onclick={() => resetSurvey(survey)}>Reset</button>
                  </td>
                </tr>
              {:else}
                {#if data}<tr><td colspan="4" class="py-6 text-center text-soft">No survey</td></tr>{/if}
              {/each}
            </tbody>
          </table>
        </div>
      </Panel>
    </div>
  </div>
</Page>
