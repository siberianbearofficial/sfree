import {Button, Card, CardBody, Chip, Divider, useDisclosure} from "@heroui/react";
import {LoginDialog, RegisterDialog} from "../../../features/auth";
import {apiUrl} from "../../../shared/api/client";
import {GoogleDriveIcon, TelegramIcon, S3Icon, GitHubIcon} from "../../../shared/icons";

const GITHUB_URL = "https://github.com/siberianbearofficial/sfree";

const sources = [
  {
    icon: <GoogleDriveIcon className="w-10 h-10 fill-current" />,
    name: "Google Drive",
    storage: "15 GB free",
    desc: "Connect free Google accounts as storage backends with automatic quota tracking.",
  },
  {
    icon: <TelegramIcon className="w-10 h-10 fill-current" />,
    name: "Telegram",
    storage: "Unlimited",
    desc: "Store encrypted chunks via Telegram bots with no hard storage cap.",
  },
  {
    icon: <S3Icon className="w-10 h-10 fill-current" />,
    name: "S3-Compatible",
    storage: "Varies",
    desc: "MinIO, Backblaze B2, Wasabi, or any endpoint that speaks the S3 protocol.",
  },
];

const steps = [
  {
    num: "1",
    title: "Connect sources",
    desc: "Link your Google Drive accounts, Telegram bots, or S3-compatible endpoints.",
  },
  {
    num: "2",
    title: "Create a bucket",
    desc: "Choose which sources back the bucket. SFree distributes data across them automatically.",
  },
  {
    num: "3",
    title: "Upload and access",
    desc: "Use the browser UI, REST API, or S3-compatible endpoint to manage files.",
  },
];

const valueProps = [
  {
    title: "No vendor lock-in",
    desc: "Your data lives on services you already own. SFree orchestrates — it never holds your files hostage.",
  },
  {
    title: "Combine free tiers",
    desc: "Pool storage across multiple free accounts into one unified namespace. Pay nothing for the storage itself.",
  },
  {
    title: "S3-compatible API",
    desc: "Works with existing tools and SDKs that speak S3. Drop it into workflows you already have.",
  },
  {
    title: "Open source",
    desc: "MIT-licensed and fully transparent. Self-host if you prefer, or use the hosted version.",
  },
];

export function LandingPage() {
  const login = useDisclosure();
  const register = useDisclosure();

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Nav */}
      <nav className="flex items-center justify-between px-6 py-4 max-w-6xl mx-auto">
        <span className="text-xl font-bold tracking-tight">SFree</span>
        <div className="flex items-center gap-3">
          <Button variant="light" size="sm" onPress={login.onOpen}>
            Log In
          </Button>
          <Button color="primary" size="sm" variant="flat" onPress={register.onOpen}>
            Sign Up
          </Button>
        </div>
      </nav>

      {/* Hero */}
      <section className="flex flex-col items-center justify-center px-6 pt-16 sm:pt-24 pb-20 text-center">
        <Chip variant="flat" color="warning" size="sm" className="mb-6">
          Early Access
        </Chip>
        <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold tracking-tight max-w-3xl leading-tight">
          Unify your free storage into one bucket
        </h1>
        <p className="mt-6 text-lg sm:text-xl text-default-500 max-w-2xl leading-relaxed">
          SFree combines Google Drive, Telegram, and S3-compatible services into a
          single object store with an S3-compatible API. Upload once — files are split,
          distributed, and reassembled on demand.
        </p>
        <div className="flex flex-col sm:flex-row gap-3 mt-10 justify-center w-full sm:w-auto">
          <Button
            color="primary"
            size="lg"
            className="font-semibold"
            onPress={register.onOpen}
          >
            Get Started Free
          </Button>
          <Button
            variant="bordered"
            size="lg"
            as="a"
            href={apiUrl("/auth/github")}
            startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
          >
            Continue with GitHub
          </Button>
        </div>
        <p className="mt-4 text-xs text-default-400">
          No credit card required. Free while in early access.
        </p>
      </section>

      {/* How it works */}
      <section className="px-6 py-16 sm:py-20 max-w-5xl mx-auto">
        <h2 className="text-3xl font-bold text-center mb-4">How it works</h2>
        <p className="text-default-500 text-center mb-12 max-w-xl mx-auto">
          Three steps from sign-up to a working distributed object store.
        </p>
        <div className="grid sm:grid-cols-2 md:grid-cols-3 gap-8">
          {steps.map((s) => (
            <div key={s.num} className="flex flex-col items-center text-center gap-3">
              <div className="flex items-center justify-center w-12 h-12 rounded-full bg-primary text-primary-foreground text-lg font-bold">
                {s.num}
              </div>
              <h3 className="text-xl font-semibold">{s.title}</h3>
              <p className="text-default-500">{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Supported sources */}
      <section className="px-6 py-16 sm:py-20 bg-default-50 dark:bg-default-50/30">
        <div className="max-w-5xl mx-auto">
          <h2 className="text-3xl font-bold text-center mb-4">Supported sources</h2>
          <p className="text-default-500 text-center mb-12 max-w-xl mx-auto">
            Bring the storage you already have. SFree handles the rest.
          </p>
          <div className="grid sm:grid-cols-2 md:grid-cols-3 gap-6">
            {sources.map((s) => (
              <Card key={s.name} className="border border-default-200">
                <CardBody className="flex flex-col items-center text-center gap-3 p-6">
                  <div className="text-primary">{s.icon}</div>
                  <h3 className="text-lg font-semibold">{s.name}</h3>
                  <Chip size="sm" variant="flat" color="success">{s.storage}</Chip>
                  <p className="text-default-500 text-sm">{s.desc}</p>
                </CardBody>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* Why SFree */}
      <section className="px-6 py-16 sm:py-20 max-w-5xl mx-auto">
        <h2 className="text-3xl font-bold text-center mb-4">Why SFree</h2>
        <p className="text-default-500 text-center mb-12 max-w-xl mx-auto">
          Built for developers and hobbyists who want free, flexible object storage
          without giving up control.
        </p>
        <div className="grid sm:grid-cols-2 gap-6">
          {valueProps.map((v) => (
            <div key={v.title} className="flex flex-col gap-2 p-6 rounded-xl border border-default-200">
              <h3 className="text-lg font-semibold">{v.title}</h3>
              <p className="text-default-500 text-sm leading-relaxed">{v.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Honesty section */}
      <section className="px-6 py-16 sm:py-20 bg-default-50 dark:bg-default-50/30">
        <div className="max-w-3xl mx-auto text-center">
          <h2 className="text-3xl font-bold mb-4">What to know</h2>
          <p className="text-default-500 mb-8 max-w-xl mx-auto">
            SFree is in early access. Here is what that means right now.
          </p>
          <div className="grid sm:grid-cols-2 md:grid-cols-3 gap-6 text-left">
            <div className="p-5 rounded-xl border border-default-200 bg-background">
              <h3 className="font-semibold mb-2">Not for production-critical data</h3>
              <p className="text-default-500 text-sm">
                SFree does not yet provide redundancy guarantees. Keep copies of anything you
                cannot afford to lose.
              </p>
            </div>
            <div className="p-5 rounded-xl border border-default-200 bg-background">
              <h3 className="font-semibold mb-2">APIs may change</h3>
              <p className="text-default-500 text-sm">
                The REST and S3-compatible APIs work today, but endpoints and behavior can
                still change as we iterate.
              </p>
            </div>
            <div className="p-5 rounded-xl border border-default-200 bg-background">
              <h3 className="font-semibold mb-2">Free and open source</h3>
              <p className="text-default-500 text-sm">
                MIT-licensed. You can self-host from{" "}
                <a
                  href={GITHUB_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  GitHub
                </a>{" "}
                or use the hosted version at no cost during early access.
              </p>
            </div>
          </div>
        </div>
      </section>

      <Divider />

      {/* Footer CTA */}
      <section className="flex flex-col items-center px-6 pt-16 pb-24 text-center gap-6">
        <h2 className="text-2xl sm:text-3xl font-bold">Ready to try it?</h2>
        <p className="text-default-500 max-w-md">
          Create a free account and start combining your storage in minutes.
        </p>
        <div className="flex flex-col sm:flex-row gap-3 justify-center">
          <Button
            color="primary"
            size="lg"
            className="font-semibold"
            onPress={register.onOpen}
          >
            Get Started Free
          </Button>
          <Button
            variant="bordered"
            size="lg"
            as="a"
            href={apiUrl("/auth/github")}
            startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
          >
            Continue with GitHub
          </Button>
          <Button variant="light" size="lg" onPress={login.onOpen}>
            Log In
          </Button>
        </div>
        <div className="flex items-center gap-4 mt-4 text-xs text-default-400">
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-default-500"
          >
            GitHub
          </a>
          <span>&middot;</span>
          <span>MIT License</span>
          <span>&middot;</span>
          <span>Early Access</span>
        </div>
      </section>

      <RegisterDialog
        isOpen={register.isOpen}
        onOpenChange={register.onOpenChange}
        onSwitchToLogin={() => {
          register.onClose();
          login.onOpen();
        }}
      />
      <LoginDialog
        isOpen={login.isOpen}
        onOpenChange={login.onOpenChange}
        onSwitchToRegister={() => {
          login.onClose();
          register.onOpen();
        }}
      />
    </div>
  );
}
