import { useTranslation } from "./i18n";

/** A paired zh/en copy bundle for one feature screen. */
export type Copy<T> = Record<"zh" | "en", T>;

/**
 * Pick the copy bundle for the active language.
 *
 * Prefer this over ad-hoc `language === "zh" ? … : …` ternaries: a typed
 * `Copy<T>` makes a missing translation a compile error rather than a
 * half-switched interface.
 */
export function useCopy<T>(copy: Copy<T>): T {
  const { language } = useTranslation();
  return copy[language];
}
