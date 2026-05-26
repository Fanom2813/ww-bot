import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Moon, Sun, MessageCircle } from "lucide-react";

function App() {
  const [dark, setDark] = useState(true);

  const toggleTheme = () => {
    const next = !dark;
    setDark(next);
    document.documentElement.classList.toggle("dark", next);
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 bg-background p-8 text-foreground">
      <div className="flex items-center gap-3">
        <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
          <MessageCircle className="h-6 w-6" />
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">WW Bot</h1>
          <p className="text-sm text-muted-foreground">WhatsApp AI reply assistant</p>
        </div>
      </div>

      <div className="flex items-center gap-2 rounded-full border px-3 py-1 text-sm text-muted-foreground">
        <span className="h-2 w-2 rounded-full bg-muted-foreground" />
        Not connected
      </div>

      <div className="flex gap-3">
        <Button>Link WhatsApp</Button>
        <Button variant="outline">Settings</Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={toggleTheme}
          aria-label="Toggle theme"
        >
          {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
      </div>

      <p className="text-xs text-muted-foreground">
        UI foundation ready · Tailwind v4 + shadcn/ui
      </p>
    </div>
  );
}

export default App;
