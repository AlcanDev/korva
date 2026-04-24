import { createContext, useContext, useState, type ReactNode } from 'react'
import { translations, type Lang, type Translations } from '@/i18n'

const LANG_KEY = 'korva-lang'
const EDITOR_KEY = 'korva-editor'

export type EditorKey = 'claudeCode' | 'vscode' | 'windsurf' | 'cursor' | 'jetbrains'

export interface EditorIntegration {
  name: string
  instructionFile: string
  skillsDir: string
  initFlag: string
}

export const EDITOR_INTEGRATION: Record<EditorKey, EditorIntegration> = {
  claudeCode: {
    name: 'Claude Code',
    instructionFile: 'CLAUDE.md',
    skillsDir: '~/.claude/commands/',
    initFlag: '',
  },
  vscode: {
    name: 'VS Code',
    instructionFile: '.github/copilot-instructions.md',
    skillsDir: '',
    initFlag: '--copilot',
  },
  windsurf: {
    name: 'Windsurf',
    instructionFile: '.windsurfrules',
    skillsDir: '',
    initFlag: '--windsurf',
  },
  cursor: {
    name: 'Cursor',
    instructionFile: '.cursor/rules/korva.md',
    skillsDir: '',
    initFlag: '--cursor',
  },
  jetbrains: {
    name: 'JetBrains',
    instructionFile: '.github/copilot-instructions.md',
    skillsDir: '',
    initFlag: '--copilot',
  },
}

interface I18nContextValue {
  lang: Lang
  setLang: (l: Lang) => void
  t: Translations
  editor: EditorKey
  setEditor: (e: EditorKey) => void
}

export const I18nContext = createContext<I18nContextValue | null>(null)

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    try {
      const s = localStorage.getItem(LANG_KEY)
      return s === 'en' || s === 'es' ? s : 'en'
    } catch { return 'en' }
  })

  const [editor, setEditorState] = useState<EditorKey>(() => {
    try {
      const s = localStorage.getItem(EDITOR_KEY) as EditorKey
      return s && s in EDITOR_INTEGRATION ? s : 'claudeCode'
    } catch { return 'claudeCode' }
  })

  const setLang = (l: Lang) => {
    try { localStorage.setItem(LANG_KEY, l) } catch { /* ignore */ }
    setLangState(l)
  }

  const setEditor = (e: EditorKey) => {
    try { localStorage.setItem(EDITOR_KEY, e) } catch { /* ignore */ }
    setEditorState(e)
  }

  return (
    <I18nContext.Provider value={{ lang, setLang, t: translations[lang], editor, setEditor }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used within I18nProvider')
  return ctx
}

/** Safe version — returns null when used outside I18nProvider. */
export function useI18nOptional(): I18nContextValue | null {
  return useContext(I18nContext)
}
