import type {ReactNode} from "react";
import {HeroUIProvider} from "@heroui/react";
import {ThemeProvider as NextThemesProvider} from "next-themes";

export function ThemeProvider({children}: {children: ReactNode}) {
  return (
      <HeroUIProvider>
          <NextThemesProvider attribute="class" defaultTheme="system">{children}</NextThemesProvider>
      </HeroUIProvider>
  );
}
