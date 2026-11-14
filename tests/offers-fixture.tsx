import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router-dom";
import { initializeLanguage } from "@fixture/i18n";
import { modules } from "@fixture/registry";
import { DirtyFormsProvider } from "@fixture/interaction";
import "@fixture/sdk";
import "@fixture/style";
import "@fixture/design";
import "@fixture/home";

const params = new URLSearchParams(location.search);
await initializeLanguage(params.get("lang") ?? "en");
document.documentElement.dataset.theme = params.get("theme") ?? "light";
document.documentElement.dataset.surface = params.get("surface") ?? "solid";
const style = document.createElement("link");
style.rel = "stylesheet";
style.href = "/dist/ui/index.css";
document.head.append(style);
await new Promise<void>((resolve, reject) => {
  const script = document.createElement("script");
  script.src = "/dist/ui/index.js";
  script.onload = () => resolve();
  script.onerror = () => reject(Error("Built File Drop bundle failed to load"));
  document.head.append(script);
});
const module = modules().find((item) => item.id === "file-drop");
if (!module) throw Error("File Drop did not register");
const Page = module.component;
const Indicator = module.backgroundIndicator!;
const query = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});
const router = createMemoryRouter(
  [
    {
      path: "*",
      element: (
        <DirtyFormsProvider>
          <header>
            <Indicator />
          </header>
          <main>
            <Page />
          </main>
        </DirtyFormsProvider>
      ),
    },
  ],
  { initialEntries: ["/file-drop/receive"] },
);
Object.assign(window, {
  offerFixture: {
    refresh: () => query.invalidateQueries({ queryKey: ["file-drop-state"] }),
    navigate: (path: string) => router.navigate(path),
  },
});
createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={query}>
    <RouterProvider router={router} />
  </QueryClientProvider>,
);
