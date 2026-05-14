import type { FormEvent } from 'react'
import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getApiErrorMessage } from '@/lib/api/http'
import { authApi } from '@/lib/api/public'

export function RegisterPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const inviteCode = searchParams.get('invite') ?? searchParams.get('code') ?? searchParams.get('ref') ?? ''
  const [form, setForm] = useState({
    username: '',
    email: '',
    code: '',
    password: '',
  })
  const [submitting, setSubmitting] = useState(false)
  const [sendingCode, setSendingCode] = useState(false)
  const [codeSent, setCodeSent] = useState(false)
  const [codeCooldown, setCodeCooldown] = useState(0)
  const [error, setError] = useState('')

  async function handleSendCode() {
    if (!form.email) {
      setError('请先填写邮箱')
      return
    }
    setSendingCode(true)
    setError('')
    try {
      await authApi.sendCode(form.email)
      setCodeSent(true)
      setCodeCooldown(60)
      const timer = setInterval(() => {
        setCodeCooldown((prev) => {
          if (prev <= 1) {
            clearInterval(timer)
            return 0
          }
          return prev - 1
        })
      }, 1000)
    } catch (err) {
      setError(getApiErrorMessage(err))
    } finally {
      setSendingCode(false)
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitting(true)
    setError('')

    try {
      await authApi.register({ ...form, ...(inviteCode ? { invite_code: inviteCode } : {}) })
      navigate('/login')
    } catch (err) {
      setError(getApiErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Card className="w-full max-w-xl border-border/70 bg-card/92 shadow-lg">
      <CardHeader className="flex flex-col gap-3">
        <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
          User sign up
        </p>
        <CardTitle className="text-3xl tracking-tight">创建账号</CardTitle>
      </CardHeader>
      <CardContent>
        <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <Label>用户名</Label>
            <Input
              value={form.username}
              onChange={(event) =>
                setForm((current) => ({ ...current, username: event.target.value }))
              }
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>邮箱</Label>
            <div className="flex gap-2">
              <Input
                type="email"
                value={form.email}
                onChange={(event) =>
                  setForm((current) => ({ ...current, email: event.target.value }))
                }
                placeholder="用于账号验证和登录"
              />
              <Button
                type="button"
                variant="outline"
                className="shrink-0"
                onClick={handleSendCode}
                disabled={sendingCode || codeCooldown > 0}
              >
                {sendingCode ? '发送中...' : codeCooldown > 0 ? `${codeCooldown}s` : '获取验证码'}
              </Button>
            </div>
            {codeSent ? (
              <p className="text-xs text-muted-foreground">验证码已发送到 {form.email}，有效期 5 分钟</p>
            ) : null}
          </div>
          <div className="flex flex-col gap-2">
            <Label>邮箱验证码</Label>
            <Input
              value={form.code}
              onChange={(event) =>
                setForm((current) => ({ ...current, code: event.target.value }))
              }
              placeholder="请输入收到的 6 位验证码"
              maxLength={6}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>密码</Label>
            <Input
              type="password"
              value={form.password}
              onChange={(event) =>
                setForm((current) => ({ ...current, password: event.target.value }))
              }
            />
          </div>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <Button className="w-full" type="submit" disabled={submitting}>
            {submitting ? '创建中...' : '创建账号'}
          </Button>
        </form>
        <div className="mt-5 text-sm text-muted-foreground">
          已有账号？{' '}
          <Link className="font-medium text-foreground hover:underline" to="/login">
            去登录
          </Link>
        </div>
      </CardContent>
    </Card>
  )
}
