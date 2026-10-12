//go:build !windows

package i18n

// SystemLocale is only for Windows: the other systems say it in the environment (LANG, LC_ALL, ...).
func SystemLocale() string { return "" }
