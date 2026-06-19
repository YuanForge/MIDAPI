import { useEffect, useState } from 'react'
import { publicApi } from '@/lib/api/public'

export type Plan = {
  credits: number
  amount: number
  origin_amount?: number
  desc?: string
  bonus?: number
}

export type SiteSettings = {
  siteName: string
  logoUrl: string
  tutorialMarkdown: string
  plans: Plan[]
  epayEnabled: boolean
  payApplyEnabled: boolean
  shouqianbaEnabled: boolean
  wechatPayEnabled: boolean
  alipayEnabled: boolean
  allowCustom: boolean
  noticeTitle: string
  noticeContent: string
  contactInfo: string
  qqGroupUrl: string
  wechatCsUrl: string
  qrCodeUrl: string
  headerHtml: string
  footerHtml: string
  showLowPriceKey: boolean
}

const defaultSettings: SiteSettings = {
  siteName: 'MidCode',
  logoUrl: '',
  tutorialMarkdown: '',
  plans: [],
  epayEnabled: false,
  payApplyEnabled: false,
  shouqianbaEnabled: false,
  wechatPayEnabled: true,
  alipayEnabled: true,
  allowCustom: false,
  noticeTitle: '',
  noticeContent: '',
  contactInfo: '',
  qqGroupUrl: '',
  wechatCsUrl: '',
  qrCodeUrl: '',
  headerHtml: '',
  footerHtml: '',
  showLowPriceKey: true,
}

export function useSiteSettings() {
  const [settings, setSettings] = useState<SiteSettings>(defaultSettings)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    async function load() {
      try {
        const response = await publicApi.getSettings()
        const maybeSettings = (response as { settings?: unknown }).settings
        const record: Record<string, unknown> =
          maybeSettings && typeof maybeSettings === 'object'
            ? (maybeSettings as Record<string, unknown>)
            : (response as Record<string, unknown>)
        const getString = (key: string) => (typeof record[key] === 'string' ? record[key] : '')
        const getBoolean = (key: string, defaultValue: boolean) => {
          const value = record[key]
          if (typeof value === 'boolean') return value
          if (typeof value === 'string') {
            if (value === 'true') return true
            if (value === 'false') return false
          }
          return defaultValue
        }
        const parsePlans = (raw: string): Plan[] => {
          try {
            const parsed = JSON.parse(raw || '[]')
            return Array.isArray(parsed) ? parsed : []
          } catch {
            return []
          }
        }
        setSettings({
          siteName: getString('site_name') || 'MidCode',
          logoUrl: getString('logo_url'),
          tutorialMarkdown: getString('tutorial_markdown'),
          plans: parsePlans(getString('recharge_plans')),
          epayEnabled: getBoolean('epay_enabled', false),
          payApplyEnabled: getBoolean('pay_apply_enabled', false),
          shouqianbaEnabled: getBoolean('shouqianba_enabled', false),
          wechatPayEnabled: getBoolean('wechat_pay_enabled', true),
          alipayEnabled: getBoolean('alipay_enabled', true),
          allowCustom: getBoolean('recharge_allow_custom', true),
          noticeTitle: getString('notice_title'),
          noticeContent: getString('notice_content'),
          contactInfo: getString('contact_info'),
          qqGroupUrl: getString('qq_group_url'),
          wechatCsUrl: getString('wechat_cs_url'),
          qrCodeUrl: getString('qrcode_url'),
          headerHtml: getString('header_html'),
          footerHtml: getString('footer_html'),
          showLowPriceKey: getBoolean('show_low_price_key', true),
        })
      } catch {
        setSettings(defaultSettings)
      } finally {
        setLoaded(true)
      }
    }

    void load()
  }, [])

  useEffect(() => {
    document.title = settings.siteName
  }, [settings.siteName])

  return { settings, loaded }
}
