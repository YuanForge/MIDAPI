import { useEffect, useState } from 'react'

import { PageHeader } from '@/components/shared/PageHeader'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { NativeSelect } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { userApi, type ApiKeyRecord, type UserChannel, type UserTask } from '@/lib/api/user'

export function UserVideoGenPage() {
  const [apiKeys, setApiKeys] = useState<ApiKeyRecord[]>([])
  const [channels, setChannels] = useState<UserChannel[]>([])
  const [selectedKeyId, setSelectedKeyId] = useState<number | undefined>()
  const [selectedChannelId, setSelectedChannelId] = useState<number | undefined>()
  const [error, setError] = useState('')
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('720p')
  const [aspectRatio, setAspectRatio] = useState('16:9')
  const [duration, setDuration] = useState('5')
  const [taskId, setTaskId] = useState('')
  const [taskStatus, setTaskStatus] = useState<'idle' | 'polling' | 'done' | 'failed'>('idle')
  const [taskError, setTaskError] = useState('')
  const [videoUrl, setVideoUrl] = useState('')
  const [running, setRunning] = useState(false)

  useEffect(() => {
    async function load() {
      try {
        const [keysRes, channelsRes] = await Promise.all([
          userApi.listApiKeys(),
          userApi.listChannels(),
        ])
        const nextKeys = Array.isArray(keysRes) ? keysRes : keysRes.api_keys ?? keysRes.keys ?? []
        const nextChannels = (Array.isArray(channelsRes) ? channelsRes : channelsRes.channels ?? []).filter(
          (item) => item.type === 'video'
        )
        setApiKeys(nextKeys)
        setChannels(nextChannels)
        if (nextKeys.length > 0) setSelectedKeyId(nextKeys[0].id)
        if (nextChannels.length > 0) setSelectedChannelId(nextChannels[0].id)
      } catch {
        setError('读取视频渠道或 API 密钥失败')
      }
    }

    void load()
  }, [])

  const [historyTasks, setHistoryTasks] = useState<UserTask[]>([])

  async function loadHistory() {
    try {
      const res = await userApi.listTasks({ type: 'video', status: 'done', size: 20 })
      const tasks = Array.isArray(res) ? res : (res.tasks ?? res.items ?? [])
      setHistoryTasks(tasks)
    } catch { /* ignore */ }
  }

  useEffect(() => { void loadHistory() }, [])

  function currentApiKey() {
    const key = apiKeys.find((item) => item.id === selectedKeyId)
    return key?.raw_key || key?.key || ''
  }

  function currentChannel() {
    return channels.find((item) => item.id === selectedChannelId) ?? channels[0]
  }

  // 异步任务轮询
  useEffect(() => {
    if (!taskId || taskStatus !== 'polling') return
    const key = apiKeys.find((item) => item.id === selectedKeyId)
    const apiKey = key?.raw_key || key?.key || ''
    if (!apiKey) return
    let cancelled = false

    const tick = async () => {
      try {
        const resp = await fetch(`/v1/tasks/${taskId}`, {
          headers: { Authorization: `Bearer ${apiKey}` },
        })
        if (!resp.ok) return
        const data = await resp.json() as { status?: string | number; result?: Record<string, unknown>; error_msg?: string; msg?: string }
        if (cancelled) return
        const st = data.status
        if (st === 'done' || st === 2) {
          const result = data.result ?? {}
          const url = typeof result.url === 'string' ? result.url : ''
          setVideoUrl(url)
          setTaskStatus('done')
          setRunning(false)
          void loadHistory()
        } else if (st === 'failed' || st === 3) {
          setTaskError(data.error_msg ?? data.msg ?? '生成失败')
          setTaskStatus('failed')
          setRunning(false)
        }
      } catch {
        // 忽略单次轮询失败
      }
    }

    const timer = setInterval(() => { void tick() }, 3000)
    void tick()
    return () => { cancelled = true; clearInterval(timer) }
  }, [taskId, taskStatus, apiKeys, selectedKeyId])

  async function generate() {
    if (!prompt.trim()) return
    const apiKey = currentApiKey()
    if (!apiKey) {
      setError('请选择可直接调用的 API 密钥')
      return
    }
    if (!selectedChannelId && channels.length === 0) {
      setError('当前没有可用的视频模型渠道')
      return
    }
    setRunning(true)
    setTaskId('')
    setTaskStatus('idle')
    setTaskError('')
    setVideoUrl('')
    setError('')
    try {
      const endpoint = currentChannel()?.id
        ? `/v1/video?channel_id=${currentChannel()?.id}`
        : '/v1/video'
      const response = await fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify({
          model: currentChannel()?.routing_model || currentChannel()?.name,
          prompt,
          size,
          aspect_ratio: aspectRatio,
          duration,
        }),
      })
      if (!response.ok) {
        throw new Error((await response.text()) || `请求失败 (${response.status})`)
      }
      const data = await response.json() as { task_id?: number | string }
      setTaskId(String(data.task_id ?? ''))
      setTaskStatus('polling')
    } catch (err) {
      setError(err instanceof Error ? err.message : '视频生成失败')
    } finally {
      setRunning(false)
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Video"
        title="视频生成"
        description="接入真实 `/v1/video`，支持发起生成任务并返回任务 ID。"
      />
      {error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      <div className="grid gap-4 xl:grid-cols-[320px_1fr] 2xl:grid-cols-[320px_1fr_240px]">
        <Card>
          <CardContent className="flex flex-col gap-4 p-6">
            <NativeSelect value={selectedKeyId} onChange={(event) => setSelectedKeyId(Number(event.target.value))}>
              {apiKeys.map((key) => (
                <option key={key.id} value={key.id}>{key.name || key.masked_key || key.key}</option>
              ))}
            </NativeSelect>
            <NativeSelect value={selectedChannelId} onChange={(event) => setSelectedChannelId(Number(event.target.value))}>
              {channels.map((channel) => (
                <option key={channel.id} value={channel.id}>{channel.name}</option>
              ))}
            </NativeSelect>
            {channels.length === 0 ? (
              <p className="text-sm text-muted-foreground">当前没有可用的视频模型渠道。</p>
            ) : null}
            <Textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="描述你想生成的视频" />
            <NativeSelect value={size} onChange={(event) => setSize(event.target.value)}>
              <option value="720p">720p</option>
              <option value="1080p">1080p</option>
            </NativeSelect>
            <NativeSelect value={aspectRatio} onChange={(event) => setAspectRatio(event.target.value)}>
              <option value="16:9">16:9</option>
              <option value="9:16">9:16</option>
              <option value="1:1">1:1</option>
            </NativeSelect>
            <NativeSelect value={duration} onChange={(event) => setDuration(event.target.value)}>
              <option value="5">5 秒</option>
              <option value="10">10 秒</option>
              <option value="15">15 秒</option>
            </NativeSelect>
            <Button onClick={generate} disabled={running || !prompt.trim() || !currentApiKey() || channels.length === 0}>
              {running ? '生成中...' : '生成视频'}
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-6 flex flex-col gap-4">
            {taskStatus === 'polling' ? (
              <Alert>
                <AlertDescription>
                  生成中，任务 ID：<span className="font-mono">{taskId}</span>，完成后将自动展示。
                </AlertDescription>
              </Alert>
            ) : null}
            {taskStatus === 'failed' && taskError ? (
              <Alert variant="destructive">
                <AlertDescription>{taskError}</AlertDescription>
              </Alert>
            ) : null}
            {taskId && taskStatus !== 'polling' ? (
              <p className="text-xs text-muted-foreground">任务 ID：{taskId}</p>
            ) : null}
            {videoUrl ? (
              <div className="flex flex-col gap-2">
                <video
                  controls
                  src={videoUrl}
                  className="w-full rounded-xl border border-border/70"
                >
                  <track kind="captions" />
                </video>
                <a href={videoUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-primary underline">
                  在新标签页打开视频 →
                </a>
              </div>
            ) : taskStatus === 'idle' ? (
              <p className="text-sm text-muted-foreground">提交后将在这里显示任务信息。</p>
            ) : null}
          </CardContent>
        </Card>
        <Card className="hidden 2xl:flex flex-col overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3 shrink-0">
            <span className="text-sm font-semibold">历史生成</span>
            <button type="button" onClick={() => void loadHistory()} className="text-xs text-muted-foreground hover:text-foreground">刷新</button>
          </div>
          <div className="flex-1 overflow-y-auto divide-y divide-border/60">
            {historyTasks.length === 0 ? (
              <p className="py-10 text-center text-xs text-muted-foreground">暂无历史记录</p>
            ) : (
              historyTasks.map((task) => {
                const url = (task.result?.url as string | undefined) ?? ''
                const prompt = ((task.request?.prompt as string | undefined) ?? '').slice(0, 40) || '视频生成'
                return (
                  <div key={task.id} className="flex flex-col gap-0.5 px-3 py-2.5">
                    <p className="truncate text-xs font-medium">{prompt}</p>
                    <p className="text-xs text-muted-foreground">
                      {task.created_at ? new Date(task.created_at).toLocaleDateString('zh-CN') : ''}
                    </p>
                    {url ? (
                      <a href={url} target="_blank" rel="noopener noreferrer" className="text-xs text-primary underline">
                        查看视频 →
                      </a>
                    ) : null}
                  </div>
                )
              })
            )}
          </div>
        </Card>
      </div>
    </>
  )
}
