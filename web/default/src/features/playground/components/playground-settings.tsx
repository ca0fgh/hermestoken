/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Download, RotateCcw, Sliders, Upload } from 'lucide-react'
import { useRef, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'

import type { ParameterEnabled, PlaygroundConfig } from '../types'

const NUMERIC_PARAMS: {
  key: keyof ParameterEnabled
  label: string
  step: number
}[] = [
  { key: 'temperature', label: 'Temperature', step: 0.1 },
  { key: 'top_p', label: 'Top P', step: 0.1 },
  { key: 'max_tokens', label: 'Max Tokens', step: 1 },
  { key: 'frequency_penalty', label: 'Frequency Penalty', step: 0.1 },
  { key: 'presence_penalty', label: 'Presence Penalty', step: 0.1 },
  { key: 'seed', label: 'Seed', step: 1 },
]

interface PlaygroundSettingsProps {
  config: PlaygroundConfig
  parameterEnabled: ParameterEnabled
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  onReset: () => void
  onImport: (
    config?: Partial<PlaygroundConfig>,
    parameterEnabled?: Partial<ParameterEnabled>
  ) => void
}

export function PlaygroundSettings({
  config,
  parameterEnabled,
  onConfigChange,
  onParameterEnabledChange,
  onReset,
  onImport,
}: PlaygroundSettingsProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleExport = () => {
    const payload = JSON.stringify(
      { config, parameter_enabled: parameterEnabled },
      null,
      2
    )
    const blob = new Blob([`${payload}\n`], {
      type: 'application/json;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `playground-config-${Date.now()}.json`
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
  }

  const handleImportFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    try {
      const text = await file.text()
      const parsed = JSON.parse(text)
      onImport(
        parsed?.config ?? parsed,
        parsed?.parameter_enabled ?? parsed?.parameterEnabled
      )
      toast.success(t('Config imported'))
    } catch {
      toast.error(t('Invalid config file'))
    }
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button variant='outline' size='sm' className='border font-medium' />
        }
      >
        <Sliders size={16} />
        <span className='hidden sm:inline'>{t('Parameters')}</span>
      </PopoverTrigger>
      <PopoverContent className='w-80' align='start'>
        <div className='flex flex-col gap-3'>
          {NUMERIC_PARAMS.map((param) => {
            const enabled = parameterEnabled[param.key]
            const rawValue = config[param.key]
            return (
              <div className='flex items-center gap-2' key={param.key}>
                <Switch
                  checked={enabled}
                  onCheckedChange={(v) =>
                    onParameterEnabledChange(param.key, v)
                  }
                />
                <Label className='flex-1 text-sm'>{t(param.label)}</Label>
                <Input
                  type='number'
                  step={param.step}
                  disabled={!enabled}
                  className='w-24'
                  value={rawValue === null ? '' : Number(rawValue)}
                  onChange={(e) => {
                    const next =
                      e.target.value === '' && param.key === 'seed'
                        ? null
                        : Number(e.target.value)
                    onConfigChange(
                      param.key,
                      next as PlaygroundConfig[typeof param.key]
                    )
                  }}
                />
              </div>
            )
          })}

          <div className='flex items-center gap-2'>
            <Switch
              checked={config.stream}
              onCheckedChange={(v) => onConfigChange('stream', v)}
            />
            <Label className='flex-1 text-sm'>{t('Stream')}</Label>
          </div>

          <Separator />

          <div className='grid grid-cols-3 gap-2'>
            <Button variant='outline' size='sm' onClick={handleExport}>
              <Download className='size-3.5' />
              {t('Export')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload className='size-3.5' />
              {t('Import')}
            </Button>
            <Button variant='outline' size='sm' onClick={onReset}>
              <RotateCcw className='size-3.5' />
              {t('Reset')}
            </Button>
          </div>
          <input
            ref={fileInputRef}
            type='file'
            accept='application/json'
            className='hidden'
            onChange={handleImportFile}
          />
        </div>
      </PopoverContent>
    </Popover>
  )
}
