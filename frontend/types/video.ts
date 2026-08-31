import type { EntryListItem } from './entry'
export type VideoListItem = EntryListItem & { meta: { duration_seconds?: number; place?: string } }
export type VideoDetail = VideoListItem & { content_md: string; primary_media?: { url: string; mime_type: string } }
