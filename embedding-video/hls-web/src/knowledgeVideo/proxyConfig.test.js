import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

describe('knowledge video media proxy', () => {
  it('proxies HLS media through Vite in development', () => {
    const config = readFileSync(resolve(root, 'vite.config.js'), 'utf8')
    expect(config).toContain("'/knowledge-video-media'")
  })

  it('uses the runtime API upstream for nginx production proxies', () => {
    const config = readFileSync(resolve(root, 'default.conf.template'), 'utf8')
    const upstreamVariable = '\\$\\{API_UPSTREAM\\}'
    for (const path of ['/api/', '/videos/', '/swagger/', '/knowledge-video-media/']) {
      const escapedPath = path.replaceAll('/', '\\/')
      expect(config).toMatch(new RegExp(`location ${escapedPath}\\s*\\{[\\s\\S]*?proxy_pass ${upstreamVariable};`))
    }
  })

  it('accepts large uploads without buffering them in nginx', () => {
    const config = readFileSync(resolve(root, 'default.conf.template'), 'utf8')
    const apiLocation = config.match(/location \/api\/\s*\{([\s\S]*?)\n    }/)
    expect(apiLocation?.[1]).toContain('client_max_body_size 2g;')
    expect(apiLocation?.[1]).toContain('proxy_request_buffering off;')
  })

  it('forwards byte-range headers for nginx media proxies', () => {
    const config = readFileSync(resolve(root, 'default.conf.template'), 'utf8')
    for (const path of ['/videos/', '/knowledge-video-media/']) {
      const escapedPath = path.replaceAll('/', '\\/')
      const location = config.match(new RegExp(`location ${escapedPath}\\s*\\{([\\s\\S]*?)\\n    }`))
      expect(location?.[1]).toContain('proxy_set_header Range $http_range;')
      expect(location?.[1]).toContain('proxy_set_header If-Range $http_if_range;')
    }
  })
})
