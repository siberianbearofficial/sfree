import {Button, Card, CardBody, Snippet, useDisclosure} from "@heroui/react";
import {LoginDialog, RegisterDialog} from "../../../features/auth";
import {apiUrl} from "../../../shared/api/client";
import {GoogleDriveIcon, TelegramIcon, S3Icon, GitHubIcon} from "../../../shared/icons";

const GITHUB_URL = "https://github.com/siberianbearofficial/sfree";

const sources = [
  {
    icon: <GoogleDriveIcon className="w-8 h-8 fill-current" />,
    name: "Google Drive",
    desc: "Use free 15 GB accounts as storage backends with full quota reporting.",
  },
  {
    icon: <TelegramIcon className="w-8 h-8 fill-current" />,
    name: "Telegram",
    desc: "Store chunks via Telegram bots — unlimited storage through the bot API.",
  },
  {
    icon: <S3Icon className="w-8 h-8 fill-current" />,
    name: "S3-Compatible",
    desc: "MinIO, Backblaze B2, Wasabi, or any S3-compatible endpoint.",
  },
];

const steps = [
  {
    num: "1",
    title: "Add sources",
    desc: "Connect your Google Drive, Telegram bot, or S3-compatible storage.",
  },
  {
    num: "2",
    title: "Create a bucket",
    desc: "Pick which sources back the bucket. SFree distributes chunks across them.",
  },
  {
    num: "3",
    title: "Upload & access",
    desc: "Use the REST API, S3-compatible endpoint, or the browser UI.",
  },
];

export function LandingPage() {
  const login = useDisclosure();
  const register = useDisclosure();

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Hero */}
      <section className="flex flex-col items-center justify-center px-6 pt-24 pb-20 text-center">
        <h1 className="text-5xl sm:text-6xl font-bold tracking-tight max-w-3xl">
          Free Distributed Object Storage
        </h1>
        <p className="mt-6 text-lg sm:text-xl text-default-500 max-w-2xl">
          Combine Google Drive, Telegram, and S3-compatible services into a single
          object store. Upload once — SFree splits, distributes, and reassembles.
        </p>
        <div className="flex flex-wrap gap-4 mt-10 justify-center">
          <Button
            color="primary"
            size="lg"
            as="a"
            href={apiUrl("/auth/github")}
            startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
          >
            Sign in with GitHub
          </Button>
          <Button color="primary" size="lg" variant="bordered" onPress={register.onOpen}>
            Sign Up
          </Button>
          <Button
            variant="bordered"
            size="lg"
            as="a"
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
          >
            View on GitHub
          </Button>
        </div>
      </section>

      {/* How it works */}
      <section className="px-6 py-16 max-w-5xl mx-auto">
        <h2 className="text-3xl font-bold text-center mb-12">How it works</h2>
        <div className="grid sm:grid-cols-3 gap-8">
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
      <section className="px-6 py-16 max-w-5xl mx-auto">
        <h2 className="text-3xl font-bold text-center mb-12">Supported sources</h2>
        <div className="grid sm:grid-cols-3 gap-6">
          {sources.map((s) => (
            <Card key={s.name} className="border border-default-200">
              <CardBody className="flex flex-col items-center text-center gap-4 p-6">
                <div className="text-primary">{s.icon}</div>
                <h3 className="text-lg font-semibold">{s.name}</h3>
                <p className="text-default-500 text-sm">{s.desc}</p>
              </CardBody>
            </Card>
          ))}
        </div>
      </section>

      {/* Quick start */}
      <section className="px-6 py-16 max-w-3xl mx-auto">
        <h2 className="text-3xl font-bold text-center mb-12">Quick start</h2>
        <div className="flex flex-col gap-6">
          <div>
            <p className="text-sm font-medium text-default-500 mb-2">1. Clone the repo</p>
            <Snippet hideSymbol className="w-full">
              git clone https://github.com/siberianbearofficial/sfree.git
            </Snippet>
          </div>
          <div>
            <p className="text-sm font-medium text-default-500 mb-2">2. Start the stack</p>
            <Snippet hideSymbol className="w-full">
              cd sfree && docker compose up --build
            </Snippet>
          </div>
          <div>
            <p className="text-sm font-medium text-default-500 mb-2">3. Open the UI</p>
            <Snippet hideSymbol className="w-full">
              open http://localhost:3000
            </Snippet>
          </div>
        </div>
      </section>

      {/* Footer CTA */}
      <section className="flex flex-col items-center px-6 pt-16 pb-24 text-center gap-6">
        <h2 className="text-2xl font-bold">Ready to unify your storage?</h2>
        <div className="flex flex-wrap gap-4 justify-center">
          <Button
            color="primary"
            size="lg"
            as="a"
            href={apiUrl("/auth/github")}
            startContent={<GitHubIcon className="w-5 h-5 fill-current" />}
          >
            Sign in with GitHub
          </Button>
          <Button color="primary" size="lg" variant="bordered" onPress={register.onOpen}>
            Sign Up
          </Button>
          <Button variant="bordered" size="lg" onPress={login.onOpen}>
            Log In
          </Button>
        </div>
        <a
          href={GITHUB_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-default-400 hover:text-default-500"
        >
          MIT License &middot; GitHub
        </a>
      </section>

      <RegisterDialog isOpen={register.isOpen} onOpenChange={register.onOpenChange} />
      <LoginDialog isOpen={login.isOpen} onOpenChange={login.onOpenChange} />
    </div>
  );
}
