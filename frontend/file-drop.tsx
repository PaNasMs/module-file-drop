import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import * as Dialog from '@radix-ui/react-dialog'
import { Link } from 'react-router-dom'
import { mdiFolderOutline, mdiHistory, mdiInboxArrowDownOutline } from '@mdi/js'
import { Button, DialogContent, FolderPicker, Icon, Notice, SectionNav, bytes } from '@panasms/ui'
import { registerModule } from '@panasms/runtime'
import { request } from '@panasms/client'
import { useRouteTab } from '@panasms/navigation'
import { registerTranslations, translator } from '@panasms/i18n'
import en from './locales/en.json'
import ru from './locales/ru.json'
import uk from './locales/uk.json'
import './file-drop.css'

registerTranslations('file-drop', { en, ru, uk })
const tr = translator('file-drop')

type File = { Name: string; Size: number }
type Offer = { id: string; files: File[]; pin: number; expires: string }
type Transfer = { id: string; files: File[]; status: string; started: string; finished?: string; error?: string }
type State = {
  window?: { expires: string }
  busy: boolean
  offer?: Offer
  transfer?: Transfer
  history: Transfer[]
  inbox: string
  defaultInbox: boolean
}
const stateKey = ['file-drop-state']
const api = <T,>(path: string, method = 'GET', body?: unknown) => request<T>(`module-api/file-drop/${path}`, method, body)
const view = () => api<State>('state')

function Indicator() {
  const query = useQuery({ queryKey: stateKey, queryFn: view, refetchInterval: 2000, retry: false })
  if (!query.data?.offer && !query.data?.transfer) return null
  return <Link className="file-drop-indicator" to="/file-drop/receive" aria-label={tr(query.data.offer ? 'incomingRequest' : 'receiving')}>
    <Icon path={mdiInboxArrowDownOutline} size={20} />
    <span>{tr(query.data.offer ? 'incomingRequest' : 'receiving')}</span>
  </Link>
}

function Page() {
  const [part, setPart] = useRouteTab('/file-drop', ['receive', 'transfers', 'folder'], 'receive')
  const [folderOpen, setFolderOpen] = useState(false)
  const client = useQueryClient()
  const query = useQuery({ queryKey: stateKey, queryFn: view, refetchInterval: 2000, retry: false })
  const refresh = () => void client.invalidateQueries({ queryKey: stateKey })
  const start = useMutation({ mutationFn: () => api('window', 'POST'), onSuccess: refresh })
  const stop = useMutation({ mutationFn: () => api('window', 'DELETE'), onSuccess: refresh })
  const decide = useMutation({
    mutationFn: (decision: 'accept' | 'decline') => api('offer', 'POST', { id: query.data?.offer?.id, decision }),
    onSuccess: refresh,
  })
  const save = useMutation({
    mutationFn: (path: string) => api('inbox', 'POST', { path }),
    onSuccess: () => { setFolderOpen(false); refresh() },
  })
  const offer = query.data?.offer
  return <div className="file-drop-page">
    <div className="page-heading"><div><h1>{tr('title')}</h1><p className="muted">{tr('subtitle')}</p></div></div>
    {query.error && <Notice error>{query.error.message}</Notice>}
    <div className="section-layout settings-page">
      <SectionNav label={tr('title')} tabs={false} value={part} onChange={setPart} items={[
        { id: 'receive', title: tr('receive'), icon: mdiInboxArrowDownOutline },
        { id: 'transfers', title: tr('transfers'), icon: mdiHistory },
        { id: 'folder', title: tr('myFolder'), icon: mdiFolderOutline },
      ]} />
      <main className="section-content">
        {part === 'receive' && <section className="file-drop-panel">
          <h2>{tr('receive')}</h2>
          <p>{tr('receiveHelp')}</p>
          <p className="file-drop-path">{tr('destination')}: <strong>{query.data?.inbox ?? '—'}</strong></p>
          {query.data?.window ? <>
            <Notice>{tr('discoverableUntil', { time: new Date(query.data.window.expires).toLocaleTimeString() })}</Notice>
            <Button disabled={stop.isPending || !!query.data.transfer} onClick={() => stop.mutate()}>{tr('stop')}</Button>
          </> : <>
            {query.data?.busy && <Notice>{tr('busy')}</Notice>}
            <Button className="primary" disabled={start.isPending || !query.data || query.data.busy} onClick={() => start.mutate()}>{tr('start')}</Button>
          </>}
          {query.data?.transfer && <Notice>{tr('receivingFiles', { count: query.data.transfer.files.length })}</Notice>}
          {start.error && <Notice error>{start.error.message}</Notice>}
          {stop.error && <Notice error>{stop.error.message}</Notice>}
        </section>}
        {part === 'transfers' && <section className="file-drop-panel">
          <h2>{tr('transfers')}</h2>
          {!query.data?.history.length && <p>{tr('noTransfers')}</p>}
          <ul className="file-drop-history">{query.data?.history.map(item => <li key={item.id}>
            <strong>{item.files.map(file => file.Name).join(', ')}</strong>
            <span>{tr(item.status)} · {new Date(item.started).toLocaleString()}</span>
            {item.error && <small>{item.error}</small>}
          </li>)}</ul>
        </section>}
        {part === 'folder' && <section className="file-drop-panel">
          <h2>{tr('myFolder')}</h2>
          <p>{tr('folderHelp')}</p>
          <p className="file-drop-path">{query.data?.inbox ?? '—'}</p>
          <div className="actions"><Button onClick={() => setFolderOpen(true)}>{tr('changeFolder')}</Button>
            {!query.data?.defaultInbox && <Button disabled={save.isPending} onClick={() => save.mutate('')}>{tr('useDefault')}</Button>}
          </div>
          {save.error && <Notice error>{save.error.message}</Notice>}
        </section>}
      </main>
    </div>
    <Dialog.Root open={!!offer} onOpenChange={open => { if (!open && offer && !decide.isPending) decide.mutate('decline') }}>
      <Dialog.Portal><Dialog.Overlay className="dialog-overlay" />
        <DialogContent className="settings-dialog" variant="compact" intent="confirm" dirty={false}
          header={<><Dialog.Title>{tr('incomingRequest')}</Dialog.Title><Dialog.Description>{tr('checkPin')}</Dialog.Description></>}
          footer={<div className="actions"><Button data-dialog-cancel disabled={decide.isPending} onClick={() => decide.mutate('decline')}>{tr('decline')}</Button>
            <Button className="primary" disabled={decide.isPending} onClick={() => decide.mutate('accept')}>{tr('accept')}</Button></div>}>
          <strong className="file-drop-pin">{String(offer?.pin ?? 0).padStart(4, '0')}</strong>
          <ul>{offer?.files.map((file, index) => <li key={index}>{file.Name} · {bytes(file.Size)}</li>)}</ul>
          {decide.error && <Notice error>{decide.error.message}</Notice>}
        </DialogContent>
      </Dialog.Portal>
    </Dialog.Root>
    <Dialog.Root open={folderOpen} onOpenChange={setFolderOpen}>
      <Dialog.Portal><Dialog.Overlay className="dialog-overlay" />
        <DialogContent className="settings-dialog folder-picker-dialog" variant="form" intent="edit" dirty={false}
          header={<><Dialog.Title>{tr('myFolder')}</Dialog.Title><Dialog.Description>{tr('folderHelp')}</Dialog.Description></>}>
          <FolderPicker policy="personal" initialPath={query.data?.inbox ?? ''} onChoose={path => save.mutate(path)} />
          {save.error && <Notice error>{save.error.message}</Notice>}
        </DialogContent>
      </Dialog.Portal>
    </Dialog.Root>
  </div>
}

registerModule({ id: 'file-drop', title: tr('title'), path: '/file-drop', routes: ['receive', 'transfers', 'folder'], icon: mdiInboxArrowDownOutline,
  memberAccess: true, component: Page, backgroundIndicator: Indicator })
