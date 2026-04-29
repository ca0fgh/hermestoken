/**
 * LobeHub Icon Loader
 * Dynamically load and render icons from @lobehub/icons
 *
 * Supports:
 * - Basic: "OpenAI", "OpenAI.Color"
 * - Chained properties: "OpenAI.Avatar.type={'platform'}"
 * - Size parameter: getLobeIcon("OpenAI", 20)
 */
import {
  useEffect,
  useMemo,
  useState,
  type ElementType,
  type ReactNode,
} from 'react'
import Ai360 from '@lobehub/icons/es/Ai360'
import Aws from '@lobehub/icons/es/Aws'
import Azure from '@lobehub/icons/es/Azure'
import Baidu from '@lobehub/icons/es/Baidu'
import Claude from '@lobehub/icons/es/Claude'
import Cloudflare from '@lobehub/icons/es/Cloudflare'
import Cohere from '@lobehub/icons/es/Cohere'
import Coze from '@lobehub/icons/es/Coze'
import DeepSeek from '@lobehub/icons/es/DeepSeek'
import Dify from '@lobehub/icons/es/Dify'
import Doubao from '@lobehub/icons/es/Doubao'
import FastGPT from '@lobehub/icons/es/FastGPT'
import Gemini from '@lobehub/icons/es/Gemini'
import Google from '@lobehub/icons/es/Google'
import Hunyuan from '@lobehub/icons/es/Hunyuan'
import Jina from '@lobehub/icons/es/Jina'
import Jimeng from '@lobehub/icons/es/Jimeng'
import Kling from '@lobehub/icons/es/Kling'
import Midjourney from '@lobehub/icons/es/Midjourney'
import Minimax from '@lobehub/icons/es/Minimax'
import Mistral from '@lobehub/icons/es/Mistral'
import Moonshot from '@lobehub/icons/es/Moonshot'
import Ollama from '@lobehub/icons/es/Ollama'
import OpenAI from '@lobehub/icons/es/OpenAI'
import OpenRouter from '@lobehub/icons/es/OpenRouter'
import Perplexity from '@lobehub/icons/es/Perplexity'
import Qwen from '@lobehub/icons/es/Qwen'
import Replicate from '@lobehub/icons/es/Replicate'
import SiliconCloud from '@lobehub/icons/es/SiliconCloud'
import Spark from '@lobehub/icons/es/Spark'
import Suno from '@lobehub/icons/es/Suno'
import Vidu from '@lobehub/icons/es/Vidu'
import Volcengine from '@lobehub/icons/es/Volcengine'
import Wenxin from '@lobehub/icons/es/Wenxin'
import XAI from '@lobehub/icons/es/XAI'
import Xinference from '@lobehub/icons/es/Xinference'
import Yi from '@lobehub/icons/es/Yi'
import Zhipu from '@lobehub/icons/es/Zhipu'

const lobeIconModuleCache = new Map<string, unknown>()
const lobeIconModulePromises = new Map<string, Promise<unknown | null>>()
const staticLobeIconRegistry: Record<string, unknown> = {
  Ai360,
  Aws,
  Azure,
  Baidu,
  Claude,
  Cloudflare,
  Cohere,
  Coze,
  DeepSeek,
  Dify,
  Doubao,
  FastGPT,
  Gemini,
  Google,
  Hunyuan,
  Jina,
  Jimeng,
  Kling,
  Midjourney,
  Minimax,
  Mistral,
  Moonshot,
  Ollama,
  OpenAI,
  OpenRouter,
  Perplexity,
  Qwen,
  Replicate,
  SiliconCloud,
  Spark,
  Suno,
  Vidu,
  Volcengine,
  Wenxin,
  XAI,
  Xinference,
  Yi,
  Zhipu,
}

async function loadLobeIconModule(baseKey: string): Promise<unknown | null> {
  if (!baseKey) {
    return null
  }

  if (staticLobeIconRegistry[baseKey]) {
    return staticLobeIconRegistry[baseKey]
  }

  if (lobeIconModuleCache.has(baseKey)) {
    return lobeIconModuleCache.get(baseKey) ?? null
  }

  if (lobeIconModulePromises.has(baseKey)) {
    return lobeIconModulePromises.get(baseKey) ?? null
  }

  const loadPromise = import(
    /* webpackInclude: /^\.\/(?!New)[^/]+\/index\.js$/ */
    `../../../node_modules/@lobehub/icons/es/${baseKey}/index.js`
  )
    .then((module: { default?: unknown }) => {
      const loadedModule = (module as { default?: unknown })?.default || module
      lobeIconModuleCache.set(baseKey, loadedModule)
      return loadedModule
    })
    .catch(() => {
      lobeIconModuleCache.set(baseKey, null)
      return null
    })
    .finally(() => {
      lobeIconModulePromises.delete(baseKey)
    })

  lobeIconModulePromises.set(baseKey, loadPromise)
  return loadPromise
}

/**
 * Parse a property value from string to appropriate type
 * @param raw - Raw string value
 * @returns Parsed value (boolean, number, or string)
 */
function parseValue(raw: string | undefined | null): string | number | boolean {
  if (raw == null) return true

  let v = String(raw).trim()

  // Remove curly braces
  if (v.startsWith('{') && v.endsWith('}')) {
    v = v.slice(1, -1).trim()
  }

  // Remove quotes
  if (
    (v.startsWith('"') && v.endsWith('"')) ||
    (v.startsWith("'") && v.endsWith("'"))
  ) {
    return v.slice(1, -1)
  }

  // Boolean
  if (v === 'true') return true
  if (v === 'false') return false

  // Number
  if (/^-?\d+(?:\.\d+)?$/.test(v)) return Number(v)

  // Return as string
  return v
}

function renderFallbackIcon(iconName: string, size: number): ReactNode {
  const firstLetter = iconName.charAt(0).toUpperCase() || '?'

  return (
    <div
      className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
      style={{ width: size, height: size }}
    >
      {firstLetter}
    </div>
  )
}

function resolveLobeIconRenderSpec(
  iconName: string,
  size: number,
  iconModule: unknown
) {
  const segments = iconName.split('.')
  let propStartIndex = 1
  let IconComponent = iconModule

  if (
    iconModule &&
    segments.length > 1 &&
    !segments[1].includes('=') &&
    (iconModule as Record<string, unknown>)[segments[1]]
  ) {
    IconComponent = (iconModule as Record<string, unknown>)[segments[1]]
    propStartIndex = 2
  }

  const props: Record<string, string | number | boolean> = {}

  for (let i = propStartIndex; i < segments.length; i++) {
    const seg = segments[i]
    if (!seg) continue

    const eqIdx = seg.indexOf('=')
    if (eqIdx === -1) {
      props[seg.trim()] = true
      continue
    }

    const key = seg.slice(0, eqIdx).trim()
    const valRaw = seg.slice(eqIdx + 1).trim()
    props[key] = parseValue(valRaw)
  }

  if (props.size == null && size != null) {
    props.size = size
  }

  return {
    IconComponent,
    props,
  }
}

function isRenderableIconComponent(value: unknown): boolean {
  return typeof value === 'function' || typeof value === 'object'
}

// eslint-disable-next-line react-refresh/only-export-components
function DynamicLobeHubIcon({
  iconName,
  size,
}: {
  iconName: string
  size: number
}) {
  const baseKey = useMemo(() => iconName.split('.')[0] || '', [iconName])
  const immediateIconModule = useMemo(() => {
    if (!baseKey) {
      return null
    }

    return (
      staticLobeIconRegistry[baseKey] ||
      lobeIconModuleCache.get(baseKey) ||
      null
    )
  }, [baseKey])
  const [loadedIconModule, setLoadedIconModule] = useState<{
    baseKey: string
    module: unknown | null
  }>({ baseKey: '', module: null })

  useEffect(() => {
    let cancelled = false

    if (!baseKey || immediateIconModule) {
      return undefined
    }

    void loadLobeIconModule(baseKey).then((module) => {
      if (!cancelled) {
        setLoadedIconModule({ baseKey, module })
      }
    })

    return () => {
      cancelled = true
    }
  }, [baseKey, immediateIconModule])

  const iconModule =
    immediateIconModule ??
    (loadedIconModule.baseKey === baseKey ? loadedIconModule.module : null)

  const renderSpec = useMemo(
    () => resolveLobeIconRenderSpec(iconName, size, iconModule),
    [iconModule, iconName, size]
  )

  if (!renderSpec.IconComponent) {
    return renderFallbackIcon(iconName, size)
  }

  if (!isRenderableIconComponent(renderSpec.IconComponent)) {
    return renderFallbackIcon(iconName, size)
  }

  const ResolvedIconComponent = renderSpec.IconComponent as ElementType
  return <ResolvedIconComponent {...renderSpec.props} />
}

/**
 * Get LobeHub icon component by name
 * @param iconName - Icon name/description (e.g., "OpenAI", "OpenAI.Color", "Claude.Avatar")
 * @param size - Icon size (default: 20)
 * @returns Icon component or fallback
 *
 * @example
 * getLobeIcon("OpenAI", 24)
 * getLobeIcon("OpenAI.Color", 20)
 * getLobeIcon("Claude.Avatar.type={'platform'}", 32)
 */
export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): ReactNode {
  if (!iconName || typeof iconName !== 'string') {
    return renderFallbackIcon('?', size)
  }

  const trimmedName = iconName.trim()
  if (!trimmedName) {
    return renderFallbackIcon('?', size)
  }

  return <DynamicLobeHubIcon iconName={trimmedName} size={size} />
}
