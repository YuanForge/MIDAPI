import { Outlet } from 'react-router-dom'
import { ActivityIcon, CheckCircle2Icon, CpuIcon, KeyRoundIcon, TrendingUpIcon, ZapIcon } from 'lucide-react'

import { AppLogo } from '@/components/shared/AppLogo'
import { ThemeToggle } from '@/components/shared/ThemeToggle'
import { Badge } from '@/components/ui/badge'
import { useSiteSettings } from '@/hooks/use-site-settings'

export function AuthLayout({ adminMode = false }: { adminMode?: boolean }) {
  const { settings: { siteName, logoUrl } } = useSiteSettings()
  const label = adminMode ? '管理后台' : '用户控制台'

  if (adminMode) {
    return (
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(ellipse_at_top_left,color-mix(in_oklab,var(--primary)_10%,transparent)_0%,transparent_50%),radial-gradient(ellipse_at_bottom_right,color-mix(in_oklab,var(--chart-3)_8%,transparent)_0%,transparent_50%),linear-gradient(160deg,color-mix(in_oklab,var(--background)_97%,var(--muted)),var(--background)_55%,color-mix(in_oklab,var(--background)_94%,var(--accent)))]">
        {/* 顶部装饰线 */}
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/30 to-transparent" />

        <div className="absolute right-5 top-5 sm:right-8">
          <ThemeToggle />
        </div>

        <div className="mx-auto flex min-h-screen w-full max-w-3xl items-center justify-center px-5 py-16 sm:px-8">
          <Outlet />
        </div>
      </div>
    )
  }

  const headline = adminMode ? '把运营、账务和风控放进同一个清晰入口。' : '一站式 AI 接口管理平台'
  const description = adminMode
    ? '面向日常管理场景优化信息密度，减少跳转，让核心指标和待处理事项进入后台后就能被快速扫描。'
    : '统一管理多模型 API Key，实时追踪调用量与账单，让每一次模型调用都可观测、可控制。'

  const features = adminMode
    ? [
        { icon: TrendingUpIcon, title: '实时监控', desc: '请求量与延迟一目了然' },
        { icon: CheckCircle2Icon, title: '账务核对', desc: '统一视图，无需跨平台' },
        { icon: KeyRoundIcon, title: '权限管理', desc: '精细到 Key 级别的控制' },
      ]
    : [
        { icon: KeyRoundIcon, title: 'API Key 集中管理', desc: '创建、撤销与权限配置' },
        { icon: TrendingUpIcon, title: '调用统计', desc: '实时请求量与 Token 用量' },
        { icon: CpuIcon, title: '多模型支持', desc: 'OpenAI · Claude · Gemini' },
      ]

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(ellipse_at_top_left,color-mix(in_oklab,var(--primary)_10%,transparent)_0%,transparent_50%),radial-gradient(ellipse_at_bottom_right,color-mix(in_oklab,var(--chart-3)_8%,transparent)_0%,transparent_50%),linear-gradient(160deg,color-mix(in_oklab,var(--background)_97%,var(--muted)),var(--background)_55%,color-mix(in_oklab,var(--background)_94%,var(--accent)))]">
      {/* 顶部装饰线 */}
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/30 to-transparent" />

      <div className="mx-auto flex min-h-screen w-full max-w-7xl flex-col px-5 py-5 sm:px-8 lg:px-10">
        {/* 移动端顶栏 */}
        <div className="flex items-center justify-between py-3 lg:hidden">
          <AppLogo siteName={siteName} logoUrl={logoUrl} label={label} />
          <ThemeToggle />
        </div>

        <div className="grid flex-1 items-center gap-10 py-6 lg:grid-cols-[1.1fr_0.9fr] lg:gap-16 lg:py-10">
          {/* 左侧宣传面板 */}
          <section className="hidden min-h-[640px] flex-col justify-between lg:flex">
            {/* Logo + 主题切换 */}
            <div className="flex items-center justify-between">
              <AppLogo siteName={siteName} logoUrl={logoUrl} label={label} />
              <ThemeToggle />
            </div>

            {/* 核心文案 */}
            <div className="flex flex-col gap-10">
              <div className="flex flex-col gap-5">
                <Badge variant="secondary" className="w-fit gap-1.5 px-3 py-1 text-xs font-medium">
                  <ZapIcon className="size-3 text-primary" />
                  FanAPI Console
                </Badge>
                <h1 className="max-w-lg text-[2.6rem] font-semibold leading-[1.18] tracking-[-0.02em] text-foreground">
                  {headline}
                </h1>
                <p className="max-w-md text-base leading-[1.75] text-muted-foreground">
                  {description}
                </p>
              </div>

              {/* 特性列表 */}
              <div className="flex flex-col gap-2">
                {features.map((item) => (
                  <div key={item.title} className="flex items-center gap-3">
                    <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-primary/8">
                      <item.icon className="size-3.5 text-primary/70" />
                    </div>
                    <span className="text-sm text-muted-foreground">
                      <span className="font-medium text-foreground/80">{item.title}</span>
                      {' — '}{item.desc}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* 底部状态卡 */}
            <div className="overflow-hidden rounded-2xl border border-border/60 bg-card/70 shadow-sm backdrop-blur-sm">
              <div className="flex items-center justify-between border-b border-border/50 px-5 py-4">
                <div>
                  <p className="text-sm font-semibold text-foreground">
                    {adminMode ? '平台运营概览' : '账户使用概览'}
                  </p>
                  <p className="text-xs text-muted-foreground">最近 24 小时</p>
                </div>
                <Badge variant="outline" className="gap-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40">
                  <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse" />
                  Online
                </Badge>
              </div>
              <div className="px-5 py-4">
                <div className="grid grid-cols-[1fr_0.72fr] gap-3">
                  {/* 柱状图卡片 */}
                  <div className="rounded-xl bg-muted/40 px-4 pb-3 pt-4">
                    <div className="mb-4 flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <ActivityIcon className="size-3.5 text-muted-foreground" />
                        <span className="text-xs font-medium text-muted-foreground">请求趋势</span>
                      </div>
                      <span className="rounded-full bg-chart-2/20 px-2 py-0.5 text-[10px] font-medium text-chart-2">
                        Stable
                      </span>
                    </div>
                    <div className="flex h-24 items-end gap-1.5">
                      {[38, 55, 44, 68, 52, 82, 66, 90, 74, 96].map((height, index) => (
                        <div
                          key={index}
                          className="flex-1 rounded-t-[3px] bg-primary/75"
                          style={{ height: `${height}%` }}
                        />
                      ))}
                    </div>
                  </div>
                  {/* 可用性卡片 */}
                  <div className="flex flex-col justify-between rounded-xl bg-muted/40 p-4">
                    <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10">
                      <ZapIcon className="size-4 text-primary" />
                    </div>
                    <div>
                      <p className="text-[1.9rem] font-semibold leading-none tracking-tight text-foreground">
                        {adminMode ? '99.9%' : '24/7'}
                      </p>
                      <p className="mt-1.5 text-xs text-muted-foreground">
                        {adminMode ? '服务可用性' : '接口可用'}
                      </p>
                    </div>
                  </div>
                </div>
                {/* 底部色条 */}
                <div className="mt-3 grid grid-cols-3 gap-2">
                  <div className="h-1 rounded-full bg-chart-2/60" />
                  <div className="h-1 rounded-full bg-primary/50" />
                  <div className="h-1 rounded-full bg-chart-5/50" />
                </div>
              </div>
            </div>
          </section>

          {/* 右侧登录表单 */}
          <div className="flex items-center justify-center lg:justify-end">
            <Outlet />
          </div>
        </div>
      </div>
    </div>
  )
}
