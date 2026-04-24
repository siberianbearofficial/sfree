import {Button, Card, CardBody} from "@heroui/react";

type StepStatus = "done" | "current" | "upcoming";

type Step = {
  number: number;
  title: string;
  description: string;
  status: StepStatus;
  ctaLabel?: string;
  onCta?: () => void;
};

function CheckIcon({className}: {className?: string}) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function StepIndicator({step}: {step: Step}) {
  if (step.status === "done") {
    return (
      <div className="w-8 h-8 rounded-full bg-success flex items-center justify-center shrink-0">
        <CheckIcon className="w-4 h-4 text-success-foreground" />
      </div>
    );
  }
  if (step.status === "current") {
    return (
      <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center shrink-0">
        <span className="text-sm font-bold text-primary-foreground">{step.number}</span>
      </div>
    );
  }
  return (
    <div className="w-8 h-8 rounded-full bg-default-200 flex items-center justify-center shrink-0">
      <span className="text-sm font-bold text-default-500">{step.number}</span>
    </div>
  );
}

function StepRow({step, isLast}: {step: Step; isLast: boolean}) {
  return (
    <div className="flex gap-4">
      <div className="flex flex-col items-center">
        <StepIndicator step={step} />
        {!isLast && (
          <div className={`w-0.5 flex-1 my-1 ${step.status === "done" ? "bg-success" : "bg-default-200"}`} />
        )}
      </div>
      <div className={`pb-6 ${isLast ? "pb-0" : ""}`}>
        <p className={`font-semibold ${step.status === "upcoming" ? "text-default-400" : ""}`}>
          {step.title}
        </p>
        <p className={`text-sm mt-0.5 ${step.status === "upcoming" ? "text-default-300" : "text-default-500"}`}>
          {step.description}
        </p>
        {step.status === "current" && step.ctaLabel && step.onCta && (
          <Button size="sm" color="primary" className="mt-2" onPress={step.onCta}>
            {step.ctaLabel}
          </Button>
        )}
      </div>
    </div>
  );
}

type Props = {
  hasSources: boolean;
  hasBuckets: boolean;
  onAddSource: () => void;
  onAddBucket: () => void;
  onGoToBucket: () => void;
};

export function OnboardingHero({hasSources, hasBuckets, onAddSource, onAddBucket, onGoToBucket}: Props) {
  const steps: Step[] = [
    {
      number: 1,
      title: "Connect a source",
      description: hasSources
        ? "Source connected."
        : "Link a Google Drive, Telegram, or S3 source so SFree knows where your files live.",
      status: hasSources ? "done" : "current",
      ctaLabel: "Add Source",
      onCta: onAddSource,
    },
    {
      number: 2,
      title: "Create a bucket",
      description: hasBuckets
        ? "Bucket created."
        : "Buckets give you S3-compatible access to upload and manage files.",
      status: hasBuckets ? "done" : hasSources ? "current" : "upcoming",
      ctaLabel: "Create Bucket",
      onCta: onAddBucket,
    },
    {
      number: 3,
      title: "Upload your first file",
      description: "Drop a file into your bucket to see SFree in action.",
      status: hasSources && hasBuckets ? "current" : "upcoming",
      ctaLabel: "Go to Bucket",
      onCta: onGoToBucket,
    },
  ];

  return (
    <Card>
      <CardBody className="p-6">
        <h2 className="text-xl font-bold mb-1">Get started with SFree</h2>
        <p className="text-sm text-default-500 mb-5">
          Three steps to your first upload.
        </p>
        <div className="flex flex-col">
          {steps.map((step, i) => (
            <StepRow key={step.number} step={step} isLast={i === steps.length - 1} />
          ))}
        </div>
      </CardBody>
    </Card>
  );
}
