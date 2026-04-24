import { en } from './en'
import { es } from './es'

export type Lang = 'en' | 'es'
export type Translations = typeof en

const translations: Record<Lang, Translations> = {
  en,
  es: es as unknown as Translations,
}

export { translations }
export { en, es }
