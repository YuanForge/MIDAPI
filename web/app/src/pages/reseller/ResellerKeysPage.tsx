import { useState } from 'react'
import { CopyIcon, KeySquareIcon, PlusIcon } from 'lucide-react'

import { PageHeader } from '@/components/shared/PageHeader'
import { TableEmpty } from '@/components/shared/TableEmpty'
import { TableSkeleton } from '@/components/shared/TableSkeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect } from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { copyToClipboard } from '@/lib/clipboard'
import { getApiErrorMessage } from '@/lib/api/http'
import { resellerApi, type ResellerKey } from '@/lib/api/reseller'
import { useAsync } from '@/hooks/use-async'

function formatTime(value?: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-'
}

export function ResellerKeysPage() {
  const { data: keys, loading, error: loadError, reload } = useAsync(async () => {
    const response = await resellerApi.getKeys()
    return Array.isArray(response) ? response : response.keys ?? response.items ?? []
  }, [] as ResellerKey[])

  const [open, setOpen] = useState(false)
  const [name, setName] = useState('代理站 API Key')
  const [keyType, setKeyType] = useState<'low_price' | 'stable'>('low_price')
  const [createdKey, setCreatedKey] = useState('')
  const [mutError, setMutError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const error = loadError || mutError

  function resetDialog(nextOpen: boolean) {
    setOpen(nextOpen)
    if (!nextOpen) {
      setName('代理站 API Key')
      setKeyType('low_price')
      setCreatedKey('')
      setMutError('')
    }
  }

  async function submit() {
    if (!name.trim()) {
      setMutError('请输入 Key 名称')
      return
    }
    setSubmitting(true)
    setMutError('')
    try {
      const response = await resellerApi.createKey({ name: name.trim(), key_type: keyType })
      setCreatedKey(response.key ?? '')
      reload()
    } catch (err) {
      setMutError(getApiErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Reseller"
        title="API Key"
        description="代理站必须先绑定代理商 Key，创建代理站后会用它对接主站 API 和同步售价。"
        actions={
          <>
            {error ? <Button size="sm" variant="outline" onClick={reload}>重试</Button> : null}
            <Button onClick={() => setOpen(true)}>
              <PlusIcon data-icon="inline-start" />
              生成 Key
            </Button>
          </>
        }
      />
      {error ? (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}
      <Card className="overflow-hidden">
        <Table className="min-w-[860px]">
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>名称</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>Key 前缀</TableHead>
              <TableHead>绑定站点</TableHead>
              <TableHead>最后使用</TableHead>
              <TableHead>创建时间</TableHead>
              <TableHead>状态</TableHead>
            </TableRow>
          </TableHeader>
          {loading ? (
            <TableSkeleton cols={8} />
          ) : (
            <TableBody>
              {keys.length === 0 ? (
                <TableEmpty
                  cols={8}
                  Icon={KeySquareIcon}
                  title="还没有 API Key"
                  description="请先生成 Key，再创建代理站。"
                />
              ) : (
                keys.map((row, index) => (
                  <TableRow key={row.id ?? index}>
                    <TableCell>{row.id ?? '-'}</TableCell>
                    <TableCell>{row.name ?? '-'}</TableCell>
                    <TableCell>{row.key_type === 'stable' ? '稳定优先' : '低价优先'}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {row.key_prefix ?? '-'}
                    </TableCell>
                    <TableCell>{row.site_count ?? 0}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{formatTime(row.last_used_at)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">{formatTime(row.created_at)}</TableCell>
                    <TableCell>
                      <Badge variant={row.is_active === false ? 'secondary' : 'default'}>
                        {row.is_active === false ? '停用' : '启用'}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          )}
        </Table>
      </Card>

      <Dialog open={open} onOpenChange={resetDialog}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>生成代理商 Key</DialogTitle>
            <DialogDescription>Key 会用于代理站和主站 API 的对接，请创建后立即保存。</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>Key 名称</Label>
              <Input value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>调用策略</Label>
              <NativeSelect value={keyType} onChange={(event) => setKeyType(event.target.value as 'low_price' | 'stable')}>
                <option value="low_price">低价优先</option>
                <option value="stable">稳定优先</option>
              </NativeSelect>
            </div>
            {createdKey ? (
              <Alert>
                <AlertDescription className="space-y-3">
                  <span className="block">Key 已生成，请立即保存。</span>
                  <span className="block break-all rounded-md bg-muted p-2 font-mono text-xs">{createdKey}</span>
                  <Button size="sm" variant="outline" onClick={() => copyToClipboard(createdKey, { successMessage: '已复制 Key' })}>
                    <CopyIcon data-icon="inline-start" />
                    复制 Key
                  </Button>
                </AlertDescription>
              </Alert>
            ) : null}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => resetDialog(false)}>关闭</Button>
            <Button onClick={submit} disabled={submitting || Boolean(createdKey)}>
              {submitting ? '生成中...' : '生成'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
