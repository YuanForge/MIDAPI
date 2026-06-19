import { createHttpClient } from '@/lib/api/http'

type Role = 'user' | 'admin' | 'vendor'

export type UploadImageCategory = 'reference' | 'channel-icon' | 'site-setting' | 'payment-qr'
export type UploadVideoCategory = 'reference-video'

const clients: Record<Role, ReturnType<typeof createHttpClient>> = {
  user: createHttpClient('user'),
  admin: createHttpClient('admin'),
  vendor: createHttpClient('vendor'),
}

const imageExts = new Set(['jpg', 'jpeg', 'png', 'gif', 'webp'])
const videoExts = new Set(['mp4', 'mov', 'webm'])

function fileExt(file: File) {
  const idx = file.name.lastIndexOf('.')
  return idx >= 0 ? file.name.slice(idx + 1).toLowerCase() : ''
}

function assertUploadFile(file: File, options: { label: string; maxBytes: number; mimePrefix: string; exts: Set<string> }) {
  if (file.size <= 0) {
    throw new Error(`${options.label}不能为空`)
  }
  if (file.size > options.maxBytes) {
    throw new Error(`${options.label}不能超过 ${Math.round(options.maxBytes / 1024 / 1024)}MB`)
  }
  if (file.type && !file.type.startsWith(options.mimePrefix)) {
    throw new Error(`仅支持上传${options.label}文件`)
  }
  const ext = fileExt(file)
  if (!ext || !options.exts.has(ext)) {
    throw new Error(`不支持的${options.label}格式`)
  }
}

export async function uploadAuthedImage(role: Role, file: File, category: UploadImageCategory) {
  assertUploadFile(file, { label: '图片', maxBytes: 10 * 1024 * 1024, mimePrefix: 'image/', exts: imageExts })
  const formData = new FormData()
  formData.append('file', file)
  formData.append('category', category)
  return clients[role].post<{ url?: string }>('/upload/image', formData)
}

export async function uploadAuthedVideo(role: Role, file: File, category: UploadVideoCategory) {
  assertUploadFile(file, { label: '视频', maxBytes: 200 * 1024 * 1024, mimePrefix: 'video/', exts: videoExts })
  const formData = new FormData()
  formData.append('file', file)
  formData.append('category', category)
  return clients[role].post<{ url?: string }>('/upload/video', formData)
}
