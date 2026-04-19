export type ThemePreference = "light" | "dark" | "system";

const THEME_STORAGE_KEY = "theme-preference";

export function isThemePreference(value: string | null): value is ThemePreference {
    return value === "light" || value === "dark" || value === "system";
}

export function getThemePreference(): ThemePreference {
    const storedTheme = localStorage.getItem(THEME_STORAGE_KEY);
    return isThemePreference(storedTheme) ? storedTheme : "system";
}

export function applyTheme(theme: ThemePreference) {
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const shouldUseDark = theme === "dark" || (theme === "system" && prefersDark);

    document.documentElement.classList.toggle("dark", shouldUseDark);
}

export function setThemePreference(theme: ThemePreference) {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
    applyTheme(theme);
}