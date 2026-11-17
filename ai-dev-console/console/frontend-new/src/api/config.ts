import { get } from './client';

/** Prefix rewrite rule served by the backend (IMAGE_REWRITES env). */
export interface ImageRewrite {
  from: string;
  to: string;
}

let cache: ImageRewrite[] | null = null;
let pending: Promise<ImageRewrite[]> | null = null;

/** Load image registry rewrites once; empty list when unconfigured. */
export function loadImageRewrites(): Promise<ImageRewrite[]> {
  if (cache) return Promise.resolve(cache);
  if (!pending) {
    pending = get<ImageRewrite[]>('/config/images')
      .then((rules) => {
        cache = rules || [];
        return cache;
      })
      .catch(() => {
        cache = [];
        return cache;
      });
  }
  return pending;
}

/** Remap a canonical image address through the configured prefix rules. */
export function rewriteImage(image: string, rewrites: ImageRewrite[]): string {
  for (const r of rewrites) {
    if (r.from && image.startsWith(r.from)) {
      return r.to + image.slice(r.from.length);
    }
  }
  return image;
}
