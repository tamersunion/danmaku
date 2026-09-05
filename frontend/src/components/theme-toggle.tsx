import { MonitorIcon, MoonIcon, SunIcon } from "lucide-react";
import { useTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuLabel,
  DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export function ThemeMenuItems() {
  const { theme, setTheme } = useTheme();
  return <DropdownMenuRadioGroup value={theme} onValueChange={value => {
    if (value === "system" || value === "light" || value === "dark") setTheme(value);
  }}>
    <DropdownMenuLabel>界面主题</DropdownMenuLabel>
    <DropdownMenuRadioItem value="system"><MonitorIcon />跟随系统</DropdownMenuRadioItem>
    <DropdownMenuRadioItem value="light"><SunIcon />浅色</DropdownMenuRadioItem>
    <DropdownMenuRadioItem value="dark"><MoonIcon />深色</DropdownMenuRadioItem>
  </DropdownMenuRadioGroup>;
}

export function ThemeToggle() {
  const { theme } = useTheme();
  const Icon = theme === "system" ? MonitorIcon : theme === "dark" ? MoonIcon : SunIcon;
  return <DropdownMenu>
    <DropdownMenuTrigger render={<Button type="button" className="ml-auto" size="icon-sm" variant="ghost" aria-label="选择界面主题" />}><Icon /></DropdownMenuTrigger>
    <DropdownMenuContent align="end"><ThemeMenuItems /></DropdownMenuContent>
  </DropdownMenu>;
}
