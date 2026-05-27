import { useEffect, useState } from "react";
import { Navigate } from "react-router";
import { QRCodeSVG } from "qrcode.react";
import {
  IconCircleCheckFilled,
  IconRocket,
  IconBrandWhatsapp,
} from "@tabler/icons-react";
import type { Icon } from "@tabler/icons-react";
import { Loader2, ShieldAlert, Smartphone } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useWA } from "@/lib/useWA";

const ONBOARDED_KEY = "ww-onboarded";

interface StepDef {
  id: string;
  title: string;
  description: string;
  icon: Icon;
}

const STEPS: StepDef[] = [
  {
    id: "welcome",
    title: "Get started",
    description: "Review the risks and agree to proceed.",
    icon: IconRocket,
  },
  {
    id: "link",
    title: "Link WhatsApp",
    description: "Scan a QR code to connect your WhatsApp account.",
    icon: IconBrandWhatsapp,
  },
];

export function Onboarding() {
  const { status, qr, startPairing } = useWA();
  const [completed, setCompleted] = useState<Set<number>>(() => new Set());
  const [onboarded, setOnboarded] = useState(
    () => localStorage.getItem(ONBOARDED_KEY) === "1",
  );

  useEffect(() => {
    if (!completed.has(1) && status === "connected") {
      setCompleted((prev) => new Set(prev).add(1));
    }
  }, [status, completed]);

  if (onboarded) return <Navigate to="/" replace />;

  const activeStep = STEPS.findIndex((_, i) => !completed.has(i));
  const completedCount = completed.size;
  const allDone = completedCount === STEPS.length;

  const completeStep = (i: number) => setCompleted((prev) => new Set(prev).add(i));

  const finishOnboarding = () => {
    localStorage.setItem(ONBOARDED_KEY, "1");
    setOnboarded(true);
  };

  if (allDone) {
    finishOnboarding();
    return null;
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-background p-4">
      <div className="w-full max-w-xl">
        <img src="/logo.png" alt="WW Bot" className="mx-auto mb-4 h-12 w-12 rounded-xl" />

        <div className="mb-6">
          <h3 className="text-center font-semibold text-foreground text-lg">
            Set up WW Bot
          </h3>
          <p className="mt-1 text-center text-muted-foreground text-sm">
            Complete these steps to get your bot up and running
          </p>
          <div className="mt-4">
            <div className="flex items-center justify-between text-sm">
              <span className="text-muted-foreground">
                <span className="font-medium text-foreground">{completedCount}</span>{" "}
                of {STEPS.length} completed
              </span>
            </div>
            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-emerald-500 transition-all duration-500"
                style={{ width: `${(completedCount / STEPS.length) * 100}%` }}
              />
            </div>
          </div>
        </div>

        <div className="space-y-3">
          {STEPS.map((step, i) => {
            const isActive = i === activeStep;
            const isDone = completed.has(i);
            const StepIcon = step.icon;

            return (
              <div
                key={step.id}
                className={cn(
                  "rounded-lg border p-4 transition-colors",
                  isActive && "border-primary/30 bg-muted/50",
                  !isActive && "border-border bg-background",
                )}
              >
                <div className="flex gap-3">
                  <div className="mt-0.5 shrink-0">
                    {isDone ? (
                      <div className="flex size-7 items-center justify-center">
                        <IconCircleCheckFilled
                          aria-hidden="true"
                          className="size-7 text-emerald-500"
                        />
                      </div>
                    ) : (
                      <div
                        className={cn(
                          "flex size-7 items-center justify-center rounded-full font-semibold text-xs",
                          isActive
                            ? "bg-primary text-primary-foreground"
                            : "bg-muted text-muted-foreground",
                        )}
                      >
                        {i + 1}
                      </div>
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-start gap-3">
                      <div className="min-w-0 flex-1">
                        <p
                          className={cn(
                            "font-medium leading-6",
                            isDone && "text-muted-foreground/50 line-through",
                            isActive && "text-foreground",
                            !isActive && !isDone && "text-muted-foreground",
                          )}
                        >
                          {step.title}
                        </p>
                        {!isActive && (
                          <p className="mt-0.5 text-muted-foreground/60 text-sm leading-5">
                            {step.description}
                          </p>
                        )}
                        {isActive && step.id === "welcome" && (
                          <WelcomeContent onAccept={() => completeStep(0)} />
                        )}
                        {isActive && step.id === "link" && (
                          <LinkContent
                            status={status}
                            qr={qr}
                            onLink={startPairing}
                            onDone={() => completeStep(1)}
                          />
                        )}
                      </div>
                      <StepIcon
                        aria-hidden="true"
                        className={cn(
                          "mt-0.5 size-5 shrink-0",
                          isDone && "text-muted-foreground/30",
                          isActive && "text-primary",
                          !isActive && !isDone && "text-muted-foreground/40",
                        )}
                      />
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function WelcomeContent({ onAccept }: { onAccept: () => void }) {
  return (
    <div className="mt-2 space-y-3">
      <p className="text-muted-foreground text-sm">
        Your AI assistant replies to WhatsApp messages on your behalf — under your
        rules, never spamming.
      </p>
      <div className="flex gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm">
        <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
        <p className="text-muted-foreground">
          Linking uses an unofficial WhatsApp connection, which is against WhatsApp's
          terms and carries a real risk of the number being banned. The bot paces
          every message to stay human-like, but use a number you can afford to lose.
        </p>
      </div>
      <Button size="sm" onClick={onAccept}>
        I understand — get started
      </Button>
    </div>
  );
}

function LinkContent({
  status,
  qr,
  onLink,
  onDone,
}: {
  status: string;
  qr: string;
  onLink: () => void;
  onDone: () => void;
}) {
  const showQR = status === "pairing" && qr !== "";
  const generating = status === "pairing" && qr === "";
  const connected = status === "connected";

  return (
    <div className="mt-2 space-y-3">
      <p className="text-muted-foreground text-sm">
        WhatsApp &rarr; Settings &rarr; Linked Devices &rarr; Link a Device, then
        scan.
      </p>

      {connected ? (
        <div className="flex items-center gap-2">
          <IconCircleCheckFilled aria-hidden="true" className="size-4 text-emerald-500" />
          <span className="text-sm text-emerald-600">Connected</span>
          <Button size="sm" className="ml-auto" onClick={onDone}>
            Continue
          </Button>
        </div>
      ) : showQR ? (
        <div className="flex justify-center">
          <div className="rounded-lg bg-white p-3">
            <QRCodeSVG value={qr} size={200} level="L" />
          </div>
        </div>
      ) : generating ? (
        <div className="flex items-center gap-2 text-muted-foreground text-sm">
          <Loader2 className="h-4 w-4 animate-spin" /> Generating QR code…
        </div>
      ) : (
        <Button size="sm" className="gap-1.5" onClick={onLink}>
          <Smartphone className="h-4 w-4" /> Show QR code
        </Button>
      )}
    </div>
  );
}
