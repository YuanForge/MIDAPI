import { useEffect, useState } from 'react'
import { ConsoleLayout, userNavGroups } from '@/layouts/ConsoleLayout'
import { userApi } from '@/lib/api/user'

export function UserLayout() {
  const [navGroups, setNavGroups] = useState(userNavGroups)

  useEffect(() => {
    userApi.listChannels().then((res) => {
      const channels = Array.isArray(res) ? res : (res as { channels?: typeof res }).channels ?? []
      const hasVideo = (channels as { type?: string }[]).some((c) => c.type === 'video')
      const hasMusic = (channels as { type?: string }[]).some((c) => c.type === 'music')
      if (!hasVideo || !hasMusic) {
        setNavGroups(userNavGroups.map((g) => ({
          ...g,
          items: g.items.filter((item) => {
            if (!hasVideo && item.href === '/video-gen') return false
            if (!hasMusic && item.href === '/music-gen') return false
            return true
          }),
        })))
      }
    }).catch(() => {/* ignore, show all nav items on error */})
  }, [])

  return (
    <ConsoleLayout
      role="user"
      groups={navGroups}
    />
  )
}
